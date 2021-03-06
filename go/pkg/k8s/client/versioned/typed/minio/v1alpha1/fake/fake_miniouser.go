/*
MIT License

Copyright (c) 2020 Fumihiro Ito

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
*/
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1alpha1 "go.f110.dev/mono/go/pkg/api/minio/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeMinIOUsers implements MinIOUserInterface
type FakeMinIOUsers struct {
	Fake *FakeMinioV1alpha1
	ns   string
}

var miniousersResource = schema.GroupVersionResource{Group: "minio.f110.dev", Version: "v1alpha1", Resource: "miniousers"}

var miniousersKind = schema.GroupVersionKind{Group: "minio.f110.dev", Version: "v1alpha1", Kind: "MinIOUser"}

// Get takes name of the minIOUser, and returns the corresponding minIOUser object, and an error if there is any.
func (c *FakeMinIOUsers) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.MinIOUser, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(miniousersResource, c.ns, name), &v1alpha1.MinIOUser{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MinIOUser), err
}

// List takes label and field selectors, and returns the list of MinIOUsers that match those selectors.
func (c *FakeMinIOUsers) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.MinIOUserList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(miniousersResource, miniousersKind, c.ns, opts), &v1alpha1.MinIOUserList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1alpha1.MinIOUserList{ListMeta: obj.(*v1alpha1.MinIOUserList).ListMeta}
	for _, item := range obj.(*v1alpha1.MinIOUserList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested minIOUsers.
func (c *FakeMinIOUsers) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(miniousersResource, c.ns, opts))

}

// Create takes the representation of a minIOUser and creates it.  Returns the server's representation of the minIOUser, and an error, if there is any.
func (c *FakeMinIOUsers) Create(ctx context.Context, minIOUser *v1alpha1.MinIOUser, opts v1.CreateOptions) (result *v1alpha1.MinIOUser, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(miniousersResource, c.ns, minIOUser), &v1alpha1.MinIOUser{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MinIOUser), err
}

// Update takes the representation of a minIOUser and updates it. Returns the server's representation of the minIOUser, and an error, if there is any.
func (c *FakeMinIOUsers) Update(ctx context.Context, minIOUser *v1alpha1.MinIOUser, opts v1.UpdateOptions) (result *v1alpha1.MinIOUser, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(miniousersResource, c.ns, minIOUser), &v1alpha1.MinIOUser{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MinIOUser), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakeMinIOUsers) UpdateStatus(ctx context.Context, minIOUser *v1alpha1.MinIOUser, opts v1.UpdateOptions) (*v1alpha1.MinIOUser, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(miniousersResource, "status", c.ns, minIOUser), &v1alpha1.MinIOUser{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MinIOUser), err
}

// Delete takes name of the minIOUser and deletes it. Returns an error if one occurs.
func (c *FakeMinIOUsers) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(miniousersResource, c.ns, name), &v1alpha1.MinIOUser{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeMinIOUsers) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(miniousersResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1alpha1.MinIOUserList{})
	return err
}

// Patch applies the patch and returns the patched minIOUser.
func (c *FakeMinIOUsers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.MinIOUser, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(miniousersResource, c.ns, name, pt, data, subresources...), &v1alpha1.MinIOUser{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1alpha1.MinIOUser), err
}
