package registry

import (
	"github.com/kiosk-sh/kiosk/pkg/apis/tenancy"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/account"
	"github.com/kiosk-sh/kiosk/pkg/apiserver/registry/space"
)

func init() {
	tenancy.NewAccountRESTFunc = account.NewAccountREST
	tenancy.NewSpaceRESTFunc = space.NewSpaceREST
}
