package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/gfotev/loadout/internal/auth"
)

// createAdmin handles `loadout admin create --email <address>`. The password is typed twice at a
// prompt that doesn't echo it, or read from the first line of stdin when that's not a terminal
// (for scripts), so it never appears in the shell history or the process list.
func createAdmin(ctx context.Context, svc *auth.Service, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("admin create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	email := fs.String("email", "", "the admin's email address")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if *email == "" || fs.NArg() != 0 {
		return fmt.Errorf("%w: admin create --email <address>", errUsage)
	}

	password, err := readPassword(stdin, stderr)
	if err != nil {
		return err
	}
	u, err := svc.CreateAdmin(ctx, *email, password)
	if err != nil {
		return err
	}
	fmt.Fprintf(stdout, "created admin %s (%s)\n", u.Email, u.ID)
	return nil
}

// resetTOTP handles `loadout admin reset-totp --email <address>`, for an admin who lost both
// their phone and their recovery codes.
func resetTOTP(ctx context.Context, svc *auth.Service, args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("admin reset-totp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	email := fs.String("email", "", "the admin's email address")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %v", errUsage, err)
	}
	if *email == "" || fs.NArg() != 0 {
		return fmt.Errorf("%w: admin reset-totp --email <address>", errUsage)
	}
	if err := svc.ResetTOTP(ctx, *email); err != nil {
		return err
	}
	fmt.Fprintf(stdout, "reset TOTP for %s: they're logged out and set it up again at the next login\n", *email)
	return nil
}

func readPassword(stdin io.Reader, stderr io.Writer) (string, error) {
	f, ok := stdin.(*os.File)
	if !ok || !term.IsTerminal(int(f.Fd())) {
		line, err := bufio.NewReader(stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", fmt.Errorf("read password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	prompt := func(label string) (string, error) {
		fmt.Fprint(stderr, label)
		b, err := term.ReadPassword(int(f.Fd()))
		fmt.Fprintln(stderr)
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return string(b), nil
	}
	fmt.Fprintf(stderr, "Password (%d to %d characters).\n", auth.MinPasswordLen, auth.MaxPasswordLen)
	pw, err := prompt("Password: ")
	if err != nil {
		return "", err
	}
	again, err := prompt("Again: ")
	if err != nil {
		return "", err
	}
	if pw != again {
		return "", errors.New("the passwords don't match")
	}
	return pw, nil
}
