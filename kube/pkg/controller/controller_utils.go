package controller

import (
	"k8s.io/client-go/tools/cache"
)

var (
	KeyFunc           = cache.DeletionHandlingMetaNamespaceKeyFunc
)
