package kubecfg

import (
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"

	"github.com/ksonnet/kubecfg/utils"
)

type DeleteCmd struct {
	ClientPool       dynamic.ClientPool
	Discovery        discovery.DiscoveryInterface
	DefaultNamespace string

	GracePeriod int64

	Objs []*unstructured.Unstructured
}

func (c DeleteCmd) Run() error {
	version, err := utils.FetchVersion(c.Discovery)
	if err != nil {
		return err
	}

	sort.Sort(sort.Reverse(utils.DependencyOrder(c.Objs)))

	deleteOpts := metav1.DeleteOptions{}
	if version.Compare(1, 6) < 0 {
		// 1.5.x option
		boolFalse := false
		deleteOpts.OrphanDependents = &boolFalse
	} else {
		// 1.6.x option (NB: Background is broken)
		fg := metav1.DeletePropagationForeground
		deleteOpts.PropagationPolicy = &fg
	}
	if c.GracePeriod >= 0 {
		deleteOpts.GracePeriodSeconds = &c.GracePeriod
	}

	for _, obj := range c.Objs {
		desc := fmt.Sprintf("%s %s", utils.ResourceNameFor(c.Discovery, obj), utils.FqName(obj))
		log.Info("Deleting ", desc)

		client, err := utils.ClientForResource(c.ClientPool, c.Discovery, obj, c.DefaultNamespace)
		if err != nil {
			return err
		}

		err = client.Delete(obj.GetName(), &deleteOpts)
		if err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("Error deleting %s: %s", desc, err)
		}

		log.Debugf("Deleted object: ", obj)
	}

	return nil
}
