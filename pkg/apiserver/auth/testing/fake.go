package testing

import (
	"github.com/kiosk-sh/kiosk/pkg/apiserver/auth"
	"k8s.io/apiserver/pkg/authentication/user"
)

var _ auth.Cache = &FakeAuthCache{}

// FakeAuthCache is used for testing to replace the actual auth cache
type FakeAuthCache struct {
	UserNamespaces map[string][]string
	UserAccounts   map[string][]string
}

// NewFakeAuthCache creates a new fake auth cache
func NewFakeAuthCache() *FakeAuthCache {
	return &FakeAuthCache{
		UserNamespaces: map[string][]string{},
		UserAccounts:   map[string][]string{},
	}
}

// GetAccountsForUser ...
func (a *FakeAuthCache) GetAccountsForUser(user user.Info, verb string) ([]string, error) {
	return a.UserAccounts[user.GetName()], nil
}

// GetNamespacesForUser ...
func (a *FakeAuthCache) GetNamespacesForUser(user user.Info, verb string) ([]string, error) {
	return a.UserNamespaces[user.GetName()], nil
}

// Run ...
func (a *FakeAuthCache) Run(stop <-chan struct{}) {
	return
}
