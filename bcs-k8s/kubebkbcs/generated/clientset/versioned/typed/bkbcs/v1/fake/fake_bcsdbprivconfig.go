/*
 * Tencent is pleased to support the open source community by making Blueking Container Service available.
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */
// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	bkbcsv1 "github.com/Tencent/bk-bcs/bcs-k8s/kubebkbcs/apis/bkbcs/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
)

// FakeBcsDbPrivConfigs implements BcsDbPrivConfigInterface
type FakeBcsDbPrivConfigs struct {
	Fake *FakeBkbcsV1
	ns   string
}

var bcsdbprivconfigsResource = schema.GroupVersionResource{Group: "bkbcs", Version: "v1", Resource: "bcsdbprivconfigs"}

var bcsdbprivconfigsKind = schema.GroupVersionKind{Group: "bkbcs", Version: "v1", Kind: "BcsDbPrivConfig"}

// Get takes name of the bcsDbPrivConfig, and returns the corresponding bcsDbPrivConfig object, and an error if there is any.
func (c *FakeBcsDbPrivConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *bkbcsv1.BcsDbPrivConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(bcsdbprivconfigsResource, c.ns, name), &bkbcsv1.BcsDbPrivConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*bkbcsv1.BcsDbPrivConfig), err
}

// List takes label and field selectors, and returns the list of BcsDbPrivConfigs that match those selectors.
func (c *FakeBcsDbPrivConfigs) List(ctx context.Context, opts v1.ListOptions) (result *bkbcsv1.BcsDbPrivConfigList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(bcsdbprivconfigsResource, bcsdbprivconfigsKind, c.ns, opts), &bkbcsv1.BcsDbPrivConfigList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &bkbcsv1.BcsDbPrivConfigList{ListMeta: obj.(*bkbcsv1.BcsDbPrivConfigList).ListMeta}
	for _, item := range obj.(*bkbcsv1.BcsDbPrivConfigList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested bcsDbPrivConfigs.
func (c *FakeBcsDbPrivConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(bcsdbprivconfigsResource, c.ns, opts))

}

// Create takes the representation of a bcsDbPrivConfig and creates it.  Returns the server's representation of the bcsDbPrivConfig, and an error, if there is any.
func (c *FakeBcsDbPrivConfigs) Create(ctx context.Context, bcsDbPrivConfig *bkbcsv1.BcsDbPrivConfig, opts v1.CreateOptions) (result *bkbcsv1.BcsDbPrivConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(bcsdbprivconfigsResource, c.ns, bcsDbPrivConfig), &bkbcsv1.BcsDbPrivConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*bkbcsv1.BcsDbPrivConfig), err
}

// Update takes the representation of a bcsDbPrivConfig and updates it. Returns the server's representation of the bcsDbPrivConfig, and an error, if there is any.
func (c *FakeBcsDbPrivConfigs) Update(ctx context.Context, bcsDbPrivConfig *bkbcsv1.BcsDbPrivConfig, opts v1.UpdateOptions) (result *bkbcsv1.BcsDbPrivConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(bcsdbprivconfigsResource, c.ns, bcsDbPrivConfig), &bkbcsv1.BcsDbPrivConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*bkbcsv1.BcsDbPrivConfig), err
}

// Delete takes name of the bcsDbPrivConfig and deletes it. Returns an error if one occurs.
func (c *FakeBcsDbPrivConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(bcsdbprivconfigsResource, c.ns, name), &bkbcsv1.BcsDbPrivConfig{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakeBcsDbPrivConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(bcsdbprivconfigsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &bkbcsv1.BcsDbPrivConfigList{})
	return err
}

// Patch applies the patch and returns the patched bcsDbPrivConfig.
func (c *FakeBcsDbPrivConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *bkbcsv1.BcsDbPrivConfig, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(bcsdbprivconfigsResource, c.ns, name, pt, data, subresources...), &bkbcsv1.BcsDbPrivConfig{})

	if obj == nil {
		return nil, err
	}
	return obj.(*bkbcsv1.BcsDbPrivConfig), err
}
