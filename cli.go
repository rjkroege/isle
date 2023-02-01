package isle

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"strings"
	"time"

	"github.com/creack/pty"
	"github.com/hashicorp/go-hclog"
	"github.com/keybase/go-keychain"
	"github.com/lab47/isle/pkg/crypto/ssh"
	"github.com/lab47/isle/pkg/crypto/ssh/terminal"
	"github.com/lab47/isle/types"
	"github.com/morikuni/aec"
	"golang.org/x/sys/unix"
)

type CLI struct {
	// Name of this VM.
	Name    string
	Image   string
	Dir     string
	AsRoot  bool
	IsTerm  bool
	Console bool

	L hclog.Logger

	// Path to the VM's control socket.
	Path string
}

func (c *CLI) Shell(cmd string, stdin io.Reader, stdout io.Writer) error {
	var cfg ssh.ClientConfig
	cfg.HostKeyCallback = func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return nil
	}
	cfg.SetDefaults()

	var (
		sconn *ssh.Client
		err   error
	)

	c.L.Info("connecting to local socket")

	for i := 0; i < 100; i++ {
		if c.IsTerm {
			fmt.Printf("🐚 Connecting...%s",
				aec.EmptyBuilder.Column(0).ANSI.String(),
			)
		}
		sconn, err = ssh.Dial("unix", c.Path, &cfg)
		if err == nil {
			break
		}

		c.L.Error("error connecting to unixsocket", "error", err)
		time.Sleep(time.Second)
	}

	sess, err := sconn.NewSession()
	if err != nil {
		c.L.Error("error creating new session", "error", err)
		return err
	}

	sess.Stdout = stdout
	sess.Stderr = stdout
	sess.Stdin = stdin

	setup := sess.Extended(2)

	hostname, err := os.Hostname()
	if err != nil {
		hostname = "isle"
	} else {
		idx := strings.IndexByte(hostname, '.')
		if idx != -1 {
			hostname = hostname[:idx]
		}
	}

	// Setup _MSL_INFO block for transport to the guest.
	u, err := user.Current()
	if err == nil {
		uid, err := strconv.Atoi(u.Uid)
		if err != nil {
			return err
		}
		data, err := json.Marshal(&types.MSLInfo{
			Name:     c.Name,
			Image:    c.Image,
			Dir:      c.Dir,
			AsRoot:   c.AsRoot,
			Hostname: hostname,
			UserName: u.Username,
			UserId:   uid,
		})
		if err != nil {
			return err
		}
		sess.Setenv("_MSL_INFO", string(data))
	}

	// Get optional auth data from KeyChain.
	if credential, err := getCredential(); err == nil {
		authinfo := &types.AuthInfo{
			Username: u.Username,
			Password: credential,
		}

		data, err := json.Marshal(authinfo)
		if err != nil {
			return err
		}
		sess.Setenv("_AUTH_DATA", string(data))
	} else {
		c.L.Warn("failed to get a value from KeyChain: \"" + err.Error() + "\". Continuing without credentials.")
	}

	if c.Console {
		sess.Setenv("ISLE_CONSOLE", "1")
	}

	if lang := os.Getenv("LANG"); lang != "" {
		sess.Setenv("LANG", lang)
	}

	rows, cols, err := pty.Getsize(os.Stdout)
	if err == nil {
		err = sess.RequestPty(os.Getenv("TERM"), rows, cols, nil)
		if err != nil {
			return err
		}
	}

	if c.IsTerm {
		fmt.Print(aec.EmptyBuilder.Column(0).EraseLine(aec.EraseModes.All).ANSI.String())
	}

	sigWin := make(chan os.Signal, 1)

	go func() {
		for {
			select {
			case <-sigWin:
				rows, cols, err := pty.Getsize(os.Stdout)
				if err == nil {
					sess.WindowChange(rows, cols)
				}
			}
		}
	}()

	signal.Notify(sigWin, unix.SIGWINCH)

	if cmd == "" {
		c.L.Info("running shell")

		state, err := terminal.MakeRaw(int(os.Stdout.Fd()))
		if err == nil {
			defer terminal.Restore(int(os.Stdout.Fd()), state)
		}

		go io.Copy(os.Stderr, setup)
		err = sess.Shell()
	} else {
		go io.Copy(ioutil.Discard, setup)

		c.L.Info("running command", "command", cmd)
		err = sess.Start(cmd)
	}

	if err != nil {
		return err
	}

	return sess.Wait()
}

// readKeyChain reads a value from the mac keychain identified by service and
// accessgroup for username and returns the read value, true if there was
// a read value and an error if one occurred.
func readKeyChain(service, username, accessgroup string) ([]byte, bool, error) {
	query := keychain.NewItem()

	// Generic password type. I want this kind
	query.SetSecClass(keychain.SecClassGenericPassword)

	// The service name. I'm using isle.liqui.org. Which is sort of made-up
	query.SetService(service)

	// The name of the current user.
	query.SetAccount(username)

	// This is suppose to be the team id (from signing / notarization) with
	// .org.liqui.mkconfig appended. I have made it up. It doesn't seem to matter.
	query.SetAccessGroup(accessgroup)

	// We only want one result
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)

	results, err := keychain.QueryItem(query)
	if err != nil {
		return nil, false,
			fmt.Errorf("tried to read keychain: %s,%s,%s didn't works: %v", service, username, accessgroup, err)
	} else if len(results) != 1 {
		return nil, false, nil
	}
	return results[0].Data, true, nil
}

// getCredential gets the container registry authentication credential
// from the Mac KeyChain. Can verify that the password has been created
// with: security dump-keychain | grep linux
func getCredential() (string, error) {
	userinfo, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("can't get the user name: %v", err)
	}

	// TODO(rjk): I don't understand what the "groovy..." parameter does
	// but it doesn't seem to matter.
	data, exists, err := readKeyChain("linuxisle.liqui.org", userinfo.Username, "groovy.org.liqui.linuxisle")
	if err != nil {
		return "", fmt.Errorf("can't read credential from keychain: %v", err)
	} else if !exists {
		return "", fmt.Errorf("no keychain. Try adding a keychain login (i.e. \"New Password Item...\") application password for your account (i.e. your username) and name linuxisle.liqui.org with git credential as password")
	}

	return string(data), nil
}
