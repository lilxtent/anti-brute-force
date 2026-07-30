package cli_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/lilxtent/anti-brute-force/internal/cli"
)

type fakeClient struct {
	calls   []string
	checkOK bool
	err     error
}

func (f *fakeClient) Check(_ context.Context, login, password, ip string) (bool, error) {
	f.record("Check(%s,%s,%s)", login, password, ip)
	return f.checkOK, f.err
}

func (f *fakeClient) Reset(_ context.Context, login, ip string) error {
	f.record("Reset(%s,%s)", login, ip)
	return f.err
}

func (f *fakeClient) AddWhitelist(_ context.Context, p netip.Prefix) error {
	f.record("AddWhitelist(%s)", p)
	return f.err
}

func (f *fakeClient) RemoveWhitelist(_ context.Context, p netip.Prefix) error {
	f.record("RemoveWhitelist(%s)", p)
	return f.err
}

func (f *fakeClient) AddBlacklist(_ context.Context, p netip.Prefix) error {
	f.record("AddBlacklist(%s)", p)
	return f.err
}

func (f *fakeClient) RemoveBlacklist(_ context.Context, p netip.Prefix) error {
	f.record("RemoveBlacklist(%s)", p)
	return f.err
}

func (f *fakeClient) Health(context.Context) error {
	f.record("Health()")
	return f.err
}

func (f *fakeClient) record(format string, args ...any) {
	f.calls = append(f.calls, fmt.Sprintf(format, args...))
}

func run(fake *fakeClient, args ...string) (string, error) {
	var out bytes.Buffer
	root := cli.NewRootCmd(func(string, time.Duration) (cli.Client, error) { return fake, nil })
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestCommandsCallTheClient(t *testing.T) {
	tests := []struct {
		name string
		args []string
		call string
		out  string
	}{
		{
			name: "whitelist add",
			args: []string{"whitelist", "add", "192.1.1.0/25"},
			call: "AddWhitelist(192.1.1.0/25)",
			out:  "whitelist: added 192.1.1.0/25\n",
		},
		{
			name: "whitelist remove",
			args: []string{"whitelist", "remove", "192.1.1.0/25"},
			call: "RemoveWhitelist(192.1.1.0/25)",
			out:  "whitelist: removed 192.1.1.0/25\n",
		},
		{
			name: "blacklist add",
			args: []string{"blacklist", "add", "10.0.0.0/8"},
			call: "AddBlacklist(10.0.0.0/8)",
			out:  "blacklist: added 10.0.0.0/8\n",
		},
		{
			name: "blacklist remove",
			args: []string{"blacklist", "remove", "10.0.0.0/8"},
			call: "RemoveBlacklist(10.0.0.0/8)",
			out:  "blacklist: removed 10.0.0.0/8\n",
		},
		{
			name: "reset",
			args: []string{"reset", "--login", "bob", "--ip", "1.2.3.4"},
			call: "Reset(bob,1.2.3.4)",
			out:  "reset: cleared buckets for login=bob ip=1.2.3.4\n",
		},
		{
			name: "health",
			args: []string{"health"},
			call: "Health()",
			out:  "ok\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeClient{}

			out, err := run(fake, tt.args...)

			require.NoError(t, err)
			require.Equal(t, []string{tt.call}, fake.calls)
			require.Equal(t, tt.out, out)
		})
	}
}

func TestCheck(t *testing.T) {
	t.Run("allowed", func(t *testing.T) {
		fake := &fakeClient{checkOK: true}

		out, err := run(fake, "check", "--login", "bob", "--password", "s3cret", "--ip", "1.2.3.4")

		require.NoError(t, err)
		require.Equal(t, []string{"Check(bob,s3cret,1.2.3.4)"}, fake.calls)
		require.Equal(t, "allowed\n", out)
	})

	t.Run("denied returns ErrDenied", func(t *testing.T) {
		fake := &fakeClient{checkOK: false}

		out, err := run(fake, "check", "--login", "bob", "--password", "s3cret", "--ip", "1.2.3.4")

		require.ErrorIs(t, err, cli.ErrDenied)
		require.Equal(t, "denied\n", out)
	})
}

func TestInvalidInputMakesNoCall(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"bad subnet", []string{"whitelist", "add", "not-a-subnet"}},
		{"ip without mask", []string{"blacklist", "add", "10.0.0.1"}},
		{"too many args", []string{"whitelist", "add", "10.0.0.0/8", "10.0.0.0/9"}},
		{"missing subnet", []string{"whitelist", "add"}},
		{"bad ip in reset", []string{"reset", "--login", "bob", "--ip", "10.0.0.0/8"}},
		{"reset without ip", []string{"reset", "--login", "bob"}},
		{"reset without login", []string{"reset", "--ip", "1.2.3.4"}},
		{"check without password", []string{"check", "--login", "bob", "--ip", "1.2.3.4"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeClient{}

			_, err := run(fake, tt.args...)

			require.Error(t, err)
			require.Empty(t, fake.calls)
		})
	}
}

func TestClientErrorIsReported(t *testing.T) {
	wantErr := errors.New("server: internal error (500 Internal Server Error)")
	fake := &fakeClient{err: wantErr}

	_, err := run(fake, "health")

	require.ErrorIs(t, err, wantErr)
}

func TestClientFactoryErrorIsReported(t *testing.T) {
	wantErr := errors.New("bad address")
	root := cli.NewRootCmd(func(string, time.Duration) (cli.Client, error) { return nil, wantErr })
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"health"})

	require.ErrorIs(t, root.Execute(), wantErr)
}

func TestAddressResolution(t *testing.T) {
	tests := []struct {
		name string
		env  string
		args []string
		want string
	}{
		{"default", "", []string{"health"}, "http://localhost:8080"},
		{"env var", "http://abf:9000", []string{"health"}, "http://abf:9000"},
		{"flag", "", []string{"--addr", "http://flag:1234", "health"}, "http://flag:1234"},
		{"flag beats env", "http://abf:9000", []string{"--addr", "http://flag:1234", "health"}, "http://flag:1234"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("ABF_ADDR", tt.env)

			var gotAddr string
			var gotTimeout time.Duration
			root := cli.NewRootCmd(func(addr string, timeout time.Duration) (cli.Client, error) {
				gotAddr, gotTimeout = addr, timeout
				return &fakeClient{}, nil
			})
			root.SetOut(&bytes.Buffer{})
			root.SetArgs(tt.args)

			require.NoError(t, root.Execute())
			require.Equal(t, tt.want, gotAddr)
			require.Equal(t, 5*time.Second, gotTimeout)
		})
	}
}

func TestTimeoutFlag(t *testing.T) {
	var gotTimeout time.Duration
	root := cli.NewRootCmd(func(_ string, timeout time.Duration) (cli.Client, error) {
		gotTimeout = timeout
		return &fakeClient{}, nil
	})
	root.SetOut(&bytes.Buffer{})
	root.SetArgs([]string{"--timeout", "30s", "health"})

	require.NoError(t, root.Execute())
	require.Equal(t, 30*time.Second, gotTimeout)
}
