// Copyright 2018 Prometheus Team
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v0mimir1

import (
	"net/mail"
	"reflect"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestEmailToIsPresent(t *testing.T) {
	in := `
to: ''
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "missing to address in email config"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestEmailHeadersCollision(t *testing.T) {
	in := `
to: 'to@email.com'
headers:
  Subject: 'Alert'
  subject: 'New Alert'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)

	expected := "duplicate header \"Subject\" in email config"

	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", expected)
	}
	if err.Error() != expected {
		t.Errorf("\nexpected:\n%v\ngot:\n%v", expected, err.Error())
	}
}

func TestEmailToAllowsMultipleAdresses(t *testing.T) {
	in := `
to: 'a@example.com, ,b@example.com,c@example.com'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatal(err)
	}

	expected := []*mail.Address{
		{Address: "a@example.com"},
		{Address: "b@example.com"},
		{Address: "c@example.com"},
	}

	res, err := mail.ParseAddressList(cfg.To)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(res, expected) {
		t.Fatalf("expected %v, got %v", expected, res)
	}
}

func TestEmailDisallowMalformed(t *testing.T) {
	in := `
to: 'a@'
`
	var cfg Config
	err := yaml.UnmarshalStrict([]byte(in), &cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = mail.ParseAddressList(cfg.To)
	if err == nil {
		t.Fatalf("no error returned, expected:\n%v", "mail: no angle-addr")
	}
}
