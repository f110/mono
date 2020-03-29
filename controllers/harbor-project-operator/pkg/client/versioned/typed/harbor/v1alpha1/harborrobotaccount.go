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

package v1alpha1

import (
	"time"

	v1alpha1 "github.com/f110/tools/controllers/harbor-project-operator/pkg/api/harbor/v1alpha1"
	scheme "github.com/f110/tools/controllers/harbor-project-operator/pkg/client/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// HarborRobotAccountsGetter has a method to return a HarborRobotAccountInterface.
// A group's client should implement this interface.
type HarborRobotAccountsGetter interface {
	HarborRobotAccounts(namespace string) HarborRobotAccountInterface
}

// HarborRobotAccountInterface has methods to work with HarborRobotAccount resources.
type HarborRobotAccountInterface interface {
	Create(*v1alpha1.HarborRobotAccount) (*v1alpha1.HarborRobotAccount, error)
	Update(*v1alpha1.HarborRobotAccount) (*v1alpha1.HarborRobotAccount, error)
	UpdateStatus(*v1alpha1.HarborRobotAccount) (*v1alpha1.HarborRobotAccount, error)
	Delete(name string, options *v1.DeleteOptions) error
	DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error
	Get(name string, options v1.GetOptions) (*v1alpha1.HarborRobotAccount, error)
	List(opts v1.ListOptions) (*v1alpha1.HarborRobotAccountList, error)
	Watch(opts v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.HarborRobotAccount, err error)
	HarborRobotAccountExpansion
}

// harborRobotAccounts implements HarborRobotAccountInterface
type harborRobotAccounts struct {
	client rest.Interface
	ns     string
}

// newHarborRobotAccounts returns a HarborRobotAccounts
func newHarborRobotAccounts(c *HarborV1alpha1Client, namespace string) *harborRobotAccounts {
	return &harborRobotAccounts{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the harborRobotAccount, and returns the corresponding harborRobotAccount object, and an error if there is any.
func (c *harborRobotAccounts) Get(name string, options v1.GetOptions) (result *v1alpha1.HarborRobotAccount, err error) {
	result = &v1alpha1.HarborRobotAccount{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of HarborRobotAccounts that match those selectors.
func (c *harborRobotAccounts) List(opts v1.ListOptions) (result *v1alpha1.HarborRobotAccountList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.HarborRobotAccountList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested harborRobotAccounts.
func (c *harborRobotAccounts) Watch(opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch()
}

// Create takes the representation of a harborRobotAccount and creates it.  Returns the server's representation of the harborRobotAccount, and an error, if there is any.
func (c *harborRobotAccounts) Create(harborRobotAccount *v1alpha1.HarborRobotAccount) (result *v1alpha1.HarborRobotAccount, err error) {
	result = &v1alpha1.HarborRobotAccount{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		Body(harborRobotAccount).
		Do().
		Into(result)
	return
}

// Update takes the representation of a harborRobotAccount and updates it. Returns the server's representation of the harborRobotAccount, and an error, if there is any.
func (c *harborRobotAccounts) Update(harborRobotAccount *v1alpha1.HarborRobotAccount) (result *v1alpha1.HarborRobotAccount, err error) {
	result = &v1alpha1.HarborRobotAccount{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		Name(harborRobotAccount.Name).
		Body(harborRobotAccount).
		Do().
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().

func (c *harborRobotAccounts) UpdateStatus(harborRobotAccount *v1alpha1.HarborRobotAccount) (result *v1alpha1.HarborRobotAccount, err error) {
	result = &v1alpha1.HarborRobotAccount{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		Name(harborRobotAccount.Name).
		SubResource("status").
		Body(harborRobotAccount).
		Do().
		Into(result)
	return
}

// Delete takes name of the harborRobotAccount and deletes it. Returns an error if one occurs.
func (c *harborRobotAccounts) Delete(name string, options *v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *harborRobotAccounts) DeleteCollection(options *v1.DeleteOptions, listOptions v1.ListOptions) error {
	var timeout time.Duration
	if listOptions.TimeoutSeconds != nil {
		timeout = time.Duration(*listOptions.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Timeout(timeout).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched harborRobotAccount.
func (c *harborRobotAccounts) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.HarborRobotAccount, err error) {
	result = &v1alpha1.HarborRobotAccount{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("harborrobotaccounts").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}