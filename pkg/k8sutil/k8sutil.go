// Copyright 2016 The prometheus-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package k8sutil

import (
	"fmt"
	"net/http"
	"net/url"
	"time"

	"k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	metav1 "k8s.io/client-go/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/util/wait"
	"k8s.io/client-go/rest"
)

// WaitForTPRReady waits for a third party resource to be available
// for use.
func WaitForTPRReady(restClient rest.Interface, tprGroup, tprVersion, tprName string) error {
	return wait.Poll(3*time.Second, 30*time.Second, func() (bool, error) {
		res := restClient.Get().AbsPath("apis", tprGroup, tprVersion, tprName).Do()
		err := res.Error()
		if err != nil {
			// RESTClient returns *errors.StatusError for any status codes < 200 or > 206
			// and http.Client.Do errors are returned directly.
			if se, ok := err.(*errors.StatusError); ok {
				if se.Status().Code == http.StatusNotFound {
					return false, nil
				}
			}
			return false, err
		}

		var statusCode int
		res.StatusCode(&statusCode)
		if statusCode != http.StatusOK {
			return false, fmt.Errorf("invalid status code: %d", statusCode)
		}

		return true, nil
	})
}

// PodRunningAndReady returns whether a pod is running and each container has
// passed it's ready state.
func PodRunningAndReady(pod v1.Pod) (bool, error) {
	switch pod.Status.Phase {
	case v1.PodFailed, v1.PodSucceeded:
		return false, fmt.Errorf("pod completed")
	case v1.PodRunning:
		for _, cond := range pod.Status.Conditions {
			if cond.Type != v1.PodReady {
				continue
			}
			return cond.Status == v1.ConditionTrue, nil
		}
		return false, fmt.Errorf("pod ready condition not found")
	}
	return false, nil
}

func NewClusterConfig(host string, tlsInsecure bool, tlsConfig *rest.TLSClientConfig) (*rest.Config, error) {
	var cfg *rest.Config
	var err error

	if len(host) == 0 {
		if cfg, err = rest.InClusterConfig(); err != nil {
			return nil, err
		}
	} else {
		cfg = &rest.Config{
			Host: host,
		}
		hostURL, err := url.Parse(host)
		if err != nil {
			return nil, fmt.Errorf("error parsing host url %s : %v", host, err)
		}
		if hostURL.Scheme == "https" {
			cfg.TLSClientConfig = *tlsConfig
			cfg.Insecure = tlsInsecure
		}
	}
	cfg.QPS = 100
	cfg.Burst = 100

	return cfg, nil
}

func IsResourceNotFoundError(err error) bool {
	se, ok := err.(*errors.StatusError)
	if !ok {
		return false
	}
	if se.Status().Code == http.StatusNotFound && se.Status().Reason == metav1.StatusReasonNotFound {
		return true
	}
	return false
}
