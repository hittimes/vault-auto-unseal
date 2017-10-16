package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

type secrets struct {
	UnsealKeys []string `json:"unseal_keys"`
	RootToken  string   `json:"root_token"`
}

func (s *secrets) Validate() error {
	if s.UnsealKeys == nil || len(s.UnsealKeys) == 0 {
		return errors.New("secrets: no unseal keys found")
	}
	if s.RootToken == "" {
		return errors.New("secrets: no root token found")
	}
	return nil
}

type secretsStash struct {
	Path string
}

func (ss *secretsStash) Save(sec *secrets) error {
	fail := func(err error) error {
		return fmt.Errorf("failed to save secrets stash: %v", err)
	}

	b, err := json.Marshal(sec)
	if err != nil {
		return fail(err)
	}
	if err := os.MkdirAll(filepath.Dir(ss.Path), 0755); err != nil {
		return fail(err)
	}
	if err := ioutil.WriteFile(ss.Path, b, 0600); err != nil {
		return fail(err)
	}
	return nil
}

func (ss *secretsStash) Load() (*secrets, error) {
	fail := func(err error) error {
		return fmt.Errorf("failed to load secrets stash: %v", err)
	}

	b, err := ioutil.ReadFile(ss.Path)
	if err != nil {
		return nil, fail(err)
	}
	sec := &secrets{}
	if err := json.Unmarshal(b, sec); err != nil {
		return nil, fail(err)
	}
	return sec, sec.Validate()
}
