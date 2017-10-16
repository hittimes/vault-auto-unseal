//go:generate stringer -type=serverStatus

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/vault/api"
)

type vaultInitParams struct {
	ServerUpWaitTimeout time.Duration
	SecretShares        int
	SecretThreshold     int
	Idempotent          bool
}

func vaultInit(ctx context.Context, params vaultInitParams, ss *secretsStash) error {
	ctxWait, cancelWait := context.WithTimeout(ctx, params.ServerUpWaitTimeout)
	defer cancelWait()
	status, err := waitForServerStatus(ctxWait, nil)
	if err != nil {
		if err == context.DeadlineExceeded {
			return errors.New("gave up waiting for Vault server")
		}
		return err
	}

	if params.Idempotent {
		switch status {
		case sealed:
			fallthrough
		case standby:
			fallthrough
		case active:
			log.Printf("[INFO] Vault server is already initialised.  Nothing to do.")
			return nil
		}
	}

	cfg, err := config()
	if err != nil {
		return err
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		return err
	}
	res, err := client.Sys().Init(&api.InitRequest{
		SecretShares:    params.SecretShares,
		SecretThreshold: params.SecretThreshold,
	})
	if err != nil {
		return err
	}
	log.Printf("[INFO] Vault initialised")

	sec := &secrets{
		UnsealKeys: res.Keys,
		RootToken:  res.RootToken,
	}
	if err := ss.Save(sec); err != nil {
		return err
	}
	log.Printf("[INFO] Secrets written to %s", ss.Path)
	return nil
}

type vaultUnsealParams struct {
	ServerUpWaitTimeout time.Duration
	Idempotent          bool
}

func vaultUnseal(ctx context.Context, params vaultUnsealParams, ss *secretsStash) error {
	ctxWait, cancelWait := context.WithTimeout(ctx, params.ServerUpWaitTimeout)
	defer cancelWait()
	status, err := waitForServerStatus(ctxWait, serverStatuses{sealed, standby, active})
	if err != nil {
		if err == context.DeadlineExceeded {
			return errors.New("gave up waiting for Vault server")
		}
		return err
	}

	if params.Idempotent {
		switch status {
		case standby:
			fallthrough
		case active:
			log.Printf("[INFO] Vault server is already unsealed.  Nothing to do.")
			return nil
		}
	}

	// Load unseal key shares after the Vault server is confirmed to be
	// initialised.  This will allow two concurrent vault-auto-unseal jobs to
	// init and unseal a new Vault server.
	var sec *secrets
	var lerr error
	ctxLoad, cancelLoad := context.WithTimeout(ctx, 20*time.Second)
	defer cancelLoad()
	rerr := retry(ctxLoad, 3*time.Second, func() bool {
		sec, lerr = ss.Load()
		if lerr != nil {
			log.Printf("[INFO] %v (will retry)", err)
			return false
		}
		return true
	})
	if rerr != nil && lerr != nil {
		return fmt.Errorf("%v: %v", rerr, lerr)
	}
	if rerr != nil {
		return rerr
	}
	if err := sec.Validate(); err != nil {
		return err
	}

	cfg, err := config()
	if err != nil {
		return err
	}
	client, err := api.NewClient(cfg)
	if err != nil {
		return err
	}
	for _, share := range sec.UnsealKeys {
		res, err := client.Sys().Unseal(share)
		if err != nil {
			return err
		}
		if res.Sealed == false {
			log.Printf("[INFO] Vault unsealed")
			return nil
		}
	}
	return errors.New("exhausted saved unseal keys - Vault remains sealed")
}

type serverStatus int

const (
	unknown serverStatus = iota
	uninitialized
	sealed
	standby
	active
)

var (
	serverUp = []serverStatus{uninitialized, sealed, standby, active}
)

type serverStatuses []serverStatus

func (s serverStatuses) Has(desired serverStatus) bool {
	for _, status := range s {
		if status == desired {
			return true
		}
	}
	return false
}

func vaultStatus(ctx context.Context) (serverStatus, error) {
	cfg, err := config()
	if err != nil {
		return unknown, err
	}
	cfg.Timeout = 5 * time.Second
	client, err := api.NewClient(cfg)
	if err != nil {
		return unknown, err
	}
	res, err := client.Sys().Health()
	if err != nil {
		return unknown, err
	}
	if res.Initialized && res.Sealed {
		return sealed, nil
	}
	if res.Initialized && res.Standby {
		return standby, nil
	}
	if res.Initialized {
		return active, nil
	}
	return uninitialized, nil
}

func waitForServerStatus(ctx context.Context, until serverStatuses) (serverStatus, error) {
	if until == nil || len(until) == 0 {
		until = serverUp
	}

	log.Printf("[INFO] Waiting for Vault server...")
	var status serverStatus
	var err error
	rerr := retry(ctx, 10*time.Second, func() bool {
		status, err = vaultStatus(ctx)
		if until.Has(status) {
			return true
		}
		log.Printf("[INFO] Vault server not ready: status:%s, err:%s", status, err)
		return false
	})
	if rerr != nil {
		return unknown, rerr
	}
	return status, err
}

func config() (*api.Config, error) {
	cfg := api.DefaultConfig()
	if err := cfg.ReadEnvironment(); err != nil {
		return nil, fmt.Errorf("vault config: %v", err)
	}
	return cfg, nil
}

func retry(ctx context.Context, interval time.Duration, work func() bool) error {
	if work() {
		return nil
	}
	t := time.NewTicker(interval)
	defer t.Stop()
loop:
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if work() {
				break loop
			}
		}
	}
	return nil
}
