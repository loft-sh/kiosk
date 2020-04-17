package merge

import (
	"bytes"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/cli-runtime/pkg/resource"
)

// Merge performs a three-way-merge on the old manifests and new manifests
func (c *Client) Merge(oldManifests, newManifests string, force bool) error {
	current, err := c.Build(bytes.NewBufferString(oldManifests), false)
	if err != nil {
		return errors.Wrap(err, "unable to build kubernetes objects from current release manifest")
	}
	target, err := c.Build(bytes.NewBufferString(newManifests), true)
	if err != nil {
		return errors.Wrap(err, "unable to build kubernetes objects from new release manifest")
	}

	if !force {
		// Do a basic diff using gvk + name to figure out what new resources are being created so we can validate they don't already exist
		existingResources := make(map[string]bool)
		for _, r := range current {
			existingResources[objectKey(r)] = true
		}

		var toBeCreated ResourceList
		for _, r := range target {
			if !existingResources[objectKey(r)] {
				toBeCreated = append(toBeCreated, r)
			}
		}

		if err := existingResourceConflict(toBeCreated); err != nil {
			return errors.Wrap(err, "rendered manifests contain a new resource that already exists. Unable to continue with update")
		}
	}

	_, err = c.Update(current, target)
	return err
}

func objectKey(r *resource.Info) string {
	gvk := r.Object.GetObjectKind().GroupVersionKind()
	return fmt.Sprintf("%s/%s/%s/%s", gvk.GroupVersion().String(), gvk.Kind, r.Namespace, r.Name)
}

func existingResourceConflict(resources ResourceList) error {
	err := resources.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}

		helper := resource.NewHelper(info.Client, info.Mapping)
		existing, err := helper.Get(info.Namespace, info.Name, info.Export)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}

			return errors.Wrap(err, "could not get information about the resource")
		}

		return fmt.Errorf("existing resource conflict: namespace: %s, name: %s, existing_kind: %s, new_kind: %s", info.Namespace, info.Name, existing.GetObjectKind().GroupVersionKind(), info.Mapping.GroupVersionKind)
	})
	return err
}
