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

// Code generated by informer-gen. DO NOT EDIT.

package v2

import (
	"context"
	time "time"

	bkbcsv2 "github.com/Tencent/bk-bcs/bcs-mesos/kubebkbcsv2/apis/bkbcs/v2"
	versioned "github.com/Tencent/bk-bcs/bcs-mesos/kubebkbcsv2/client/clientset/versioned"
	internalinterfaces "github.com/Tencent/bk-bcs/bcs-mesos/kubebkbcsv2/client/informers/externalversions/internalinterfaces"
	v2 "github.com/Tencent/bk-bcs/bcs-mesos/kubebkbcsv2/client/listers/bkbcs/v2"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// FrameworkInformer provides access to a shared informer and lister for
// Frameworks.
type FrameworkInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v2.FrameworkLister
}

type frameworkInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewFrameworkInformer constructs a new informer for Framework type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFrameworkInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredFrameworkInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredFrameworkInformer constructs a new informer for Framework type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredFrameworkInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BkbcsV2().Frameworks(namespace).List(context.TODO(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.BkbcsV2().Frameworks(namespace).Watch(context.TODO(), options)
			},
		},
		&bkbcsv2.Framework{},
		resyncPeriod,
		indexers,
	)
}

func (f *frameworkInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredFrameworkInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *frameworkInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&bkbcsv2.Framework{}, f.defaultInformer)
}

func (f *frameworkInformer) Lister() v2.FrameworkLister {
	return v2.NewFrameworkLister(f.Informer().GetIndexer())
}
