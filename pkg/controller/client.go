package controller

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/coreos/kube-prometheus-controller/pkg/spec"
	"github.com/coreos/rkt/tests/testutils/logger"
	"k8s.io/client-go/1.5/pkg/api"
	"k8s.io/client-go/1.5/pkg/api/unversioned"
	"k8s.io/client-go/1.5/pkg/runtime"
	"k8s.io/client-go/1.5/pkg/runtime/serializer"
	"k8s.io/client-go/1.5/pkg/watch"
	"k8s.io/client-go/1.5/rest"
	"k8s.io/client-go/1.5/tools/cache"
)

const resyncPeriod = 5 * time.Minute

func newPrometheusRESTClient(c rest.Config) (*rest.RESTClient, error) {
	c.APIPath = "/apis"
	c.GroupVersion = &unversioned.GroupVersion{
		Group:   "prometheus.coreos.com",
		Version: "v1alpha1",
	}
	// TODO(fabxc): is this even used with our custom list/watch functions?
	c.NegotiatedSerializer = serializer.DirectCodecFactory{CodecFactory: api.Codecs}
	return rest.RESTClientFor(&c)
}

type prometheusDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *prometheusDecoder) Close() {
	d.close()
}

type prometheusEvent struct {
	Type   watch.EventType
	Object spec.Prometheus
}

func (d *prometheusDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e prometheusEvent
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}

// NewPrometheusListWatch returns a new ListWatch on the Prometheus resource.
func NewPrometheusListWatch(client *rest.RESTClient) *cache.ListWatch {
	return &cache.ListWatch{
		ListFunc: func(options api.ListOptions) (runtime.Object, error) {
			req := client.Get().
				Namespace(api.NamespaceAll).
				Resource("prometheuses").
				// VersionedParams(&options, api.ParameterCodec)
				FieldsSelectorParam(nil)

			b, err := req.DoRaw()
			if err != nil {
				return nil, err
			}
			var p spec.PrometheusList
			if err := json.Unmarshal(b, &p); err != nil {
				return nil, err
			}
			logger.Log("listres", fmt.Sprintf("%+v", p))
			return &p, nil
		},
		WatchFunc: func(options api.ListOptions) (watch.Interface, error) {
			logger.Log("msg", "called watch")
			r, err := client.Get().
				Prefix("watch").
				Namespace(api.NamespaceAll).
				Resource("prometheuses").
				// VersionedParams(&options, api.ParameterCodec).
				FieldsSelectorParam(nil).
				Stream()
			if err != nil {
				return nil, err
			}
			return watch.NewStreamWatcher(&prometheusDecoder{
				dec:   json.NewDecoder(r),
				close: r.Close,
			}), nil
		},
	}
}
