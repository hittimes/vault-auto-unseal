package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"golang.org/x/sys/unix"

	"github.com/alecthomas/kingpin"
	"github.com/hashicorp/logutils"
)

func main() {
	app := kingpin.New("vault-auto-unseal",
		"Automatically init and unseal a Vault server.\n\n"+
			"By default, this program will attempt to unseal the Vault server listening on https://127.0.0.1:8200.  An alternative server address may be supplied using the VAULT_ADDR environment variable.  Refer to upstream documentation for a full list of environmental options:\n\n"+
			"https://www.vaultproject.io/docs/commands/environment.html\n\n")

	stashPath := app.Flag("stash-file",
		"Save Vault unseal keys at this local filesystem path.  Following a successful init, this file will be created with mode 0600.").
		PlaceHolder("PATH").Required().String()
	silent := app.Flag("silent",
		"Suppress informational messages that would otherwise be written to standard error.").
		Default("false").Bool()
	serverUpWaitTimeout := app.Flag("server-up-wait-timeout",
		"Give up and exit with non-zero status if the Vault server is not reachable after this length of time.").
		Default("5m").Duration()
	idempotent := app.Flag("idempotent",
		"Exit with status zero if the Vault server is already in the desired state (initialised/unsealed).").
		Default("true").Bool()

	init := app.Command("init",
		"Initialise a new Vault server and save unseal keys for later use.")
	initSecretShares := init.Flag("secret-shares",
		"https://www.vaultproject.io/api/system/init.html#secret_shares").
		Default("1").Uint()
	initSecretThreshold := init.Flag("secret-threshold",
		"https://www.vaultproject.io/api/system/init.html#secret_threshold").
		Default("1").Uint()

	unseal := app.Command("unseal",
		"Unseal a Vault server using saved unseal keys.")

	cmd := kingpin.MustParse(app.Parse(os.Args[1:]))

	minLevel := logutils.LogLevel("INFO")
	if *silent {
		minLevel = logutils.LogLevel("ERROR")
	}
	log.SetFlags(log.Lshortfile)
	log.SetOutput(&logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"INFO", "ERROR"},
		MinLevel: minLevel,
		Writer:   os.Stderr,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	signals := make(chan os.Signal, 1)
	signal.Notify(signals, unix.SIGINT, unix.SIGTERM)
	go func() {
		select {
		case <-ctx.Done():
		case s := <-signals:
			log.Printf("%s received - terminating...", s)
			cancel()
		}
	}()

	var err error
	switch cmd {
	case init.FullCommand():
		err = vaultInit(ctx, vaultInitParams{
			ServerUpWaitTimeout: *serverUpWaitTimeout,
			SecretShares:        int(*initSecretShares),
			SecretThreshold:     int(*initSecretThreshold),
			Idempotent:          *idempotent,
		}, &secretsStash{Path: *stashPath})

	case unseal.FullCommand():
		err = vaultUnseal(ctx, vaultUnsealParams{
			ServerUpWaitTimeout: *serverUpWaitTimeout,
			Idempotent:          *idempotent,
		}, &secretsStash{Path: *stashPath})
	}
	if err != nil {
		app.Fatalf("%s", err)
	}
}
