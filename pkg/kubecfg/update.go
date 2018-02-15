package kubecfg

import (
	"encoding/json"
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/ksonnet/kubecfg/utils"
)

const (
	// GcListModeClusterScope lists all the objects by making one single call.
	// It behaves like kubectl get --all-namespaces.
	// It can fail if the user doesn't have rights to fetch at the cluster level
	// (i.e. if the user not in a clusterrolebinding).
	GcListModeClusterScope = "cluster-scope"
	// GcListModePerNamespace lists all the objects by making one specific call for each namespaces.
	// This can be necessary if the user has only a rolebinding in a few namespaces,
	// it can very slow on clusters with many namespaces.
	// Cluster scope queries for resouces that only exist at the cluster level are still performed,
	// so warning about permission errors are expected.
	GcListModePerNamespace = "per-namespace"

	// AnnotationGcTag annotation that triggers
	// garbage collection. Objects with value equal to
	// command-line flag that are *not* in config will be deleted.
	AnnotationGcTag = "kubecfg.ksonnet.io/garbage-collect-tag"

	// AnnotationGcStrategy controls gc logic.  Current values:
	// `auto` (default if absent) - do garbage collection
	// `ignore` - never garbage collect this object
	AnnotationGcStrategy = "kubecfg.ksonnet.io/garbage-collect-strategy"

	// GcStrategyAuto is the default automatic gc logic
	GcStrategyAuto = "auto"
	// GcStrategyIgnore means this object should be ignored by garbage collection
	GcStrategyIgnore = "ignore"
)

// UpdateCmd represents the update subcommand
type UpdateCmd struct {
	ClientPool       dynamic.ClientPool
	Discovery        discovery.DiscoveryInterface
	CoreV1Client     v1.CoreV1Interface
	DefaultNamespace string

	Create       bool
	GcTag        string
	GcListMode   string
	GcNsSelector string
	SkipGc       bool
	DryRun       bool
}

func (c UpdateCmd) Run(apiObjects []*unstructured.Unstructured) error {
	dryRunText := ""
	if c.DryRun {
		dryRunText = " (dry-run)"
	}

	log.Infof("Fetching schemas for %d resources", len(apiObjects))
	depOrder, err := utils.DependencyOrder(c.Discovery, apiObjects)
	if err != nil {
		return err
	}
	sort.Sort(depOrder)

	seenUids := sets.NewString()

	for _, obj := range apiObjects {
		if c.GcTag != "" {
			utils.SetMetaDataAnnotation(obj, AnnotationGcTag, c.GcTag)
		}

		desc := fmt.Sprintf("%s %s", utils.ResourceNameFor(c.Discovery, obj), utils.FqName(obj))
		log.Info("Updating ", desc, dryRunText)

		rc, err := utils.ClientForResource(c.ClientPool, c.Discovery, obj, c.DefaultNamespace)
		if err != nil {
			return err
		}

		asPatch, err := json.Marshal(obj)
		if err != nil {
			return err
		}
		var newobj metav1.Object
		if !c.DryRun {
			newobj, err = rc.Patch(obj.GetName(), types.MergePatchType, asPatch)
			log.Debugf("Patch(%s) returned (%v, %v)", obj.GetName(), newobj, err)
		} else {
			newobj, err = rc.Get(obj.GetName(), metav1.GetOptions{})
		}
		if c.Create && errors.IsNotFound(err) {
			log.Info(" Creating non-existent ", desc, dryRunText)
			if !c.DryRun {
				newobj, err = rc.Create(obj)
				log.Debugf("Create(%s) returned (%v, %v)", obj.GetName(), newobj, err)
			} else {
				newobj = obj
				err = nil
			}
		}
		if err != nil {
			// TODO: retry
			return fmt.Errorf("Error updating %s: %s", desc, err)
		}

		log.Debug("Updated object: ", diff.ObjectDiff(obj, newobj))

		// Some objects appear under multiple kinds
		// (eg: Deployment is both extensions/v1beta1
		// and apps/v1beta1).  UID is the only stable
		// identifier that links these two views of
		// the same object.
		seenUids.Insert(string(newobj.GetUID()))
	}

	if c.GcTag != "" && !c.SkipGc {
		version, err := utils.FetchVersion(c.Discovery)
		if err != nil {
			return err
		}

		nsListOptions := metav1.ListOptions{LabelSelector: c.GcNsSelector}
		err = walkObjects(c.ClientPool, c.Discovery, c.CoreV1Client, c.GcListMode, metav1.ListOptions{}, nsListOptions, func(o runtime.Object) error {
			meta, err := meta.Accessor(o)
			if err != nil {
				return err
			}
			gvk := o.GetObjectKind().GroupVersionKind()
			desc := fmt.Sprintf("%s %s (%s)", utils.ResourceNameFor(c.Discovery, o), utils.FqName(meta), gvk.GroupVersion())
			log.Debugf("Considering %v for gc", desc)
			if eligibleForGc(meta, c.GcTag) && !seenUids.Has(string(meta.GetUID())) {
				log.Info("Garbage collecting ", desc, dryRunText)
				if !c.DryRun {
					err := gcDelete(c.ClientPool, c.Discovery, &version, o)
					if err != nil {
						return err
					}
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func stringListContains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}

func gcDelete(clientpool dynamic.ClientPool, disco discovery.DiscoveryInterface, version *utils.ServerVersion, o runtime.Object) error {
	obj, err := meta.Accessor(o)
	if err != nil {
		return fmt.Errorf("Unexpected object type: %s", err)
	}

	uid := obj.GetUID()
	desc := fmt.Sprintf("%s %s", utils.ResourceNameFor(disco, o), utils.FqName(obj))

	deleteOpts := metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &uid},
	}
	if version.Compare(1, 6) < 0 {
		// 1.5.x option
		boolFalse := false
		deleteOpts.OrphanDependents = &boolFalse
	} else {
		// 1.6.x option (NB: Background is broken)
		fg := metav1.DeletePropagationForeground
		deleteOpts.PropagationPolicy = &fg
	}

	c, err := utils.ClientForResource(clientpool, disco, o, metav1.NamespaceNone)
	if err != nil {
		return err
	}

	err = c.Delete(obj.GetName(), &deleteOpts)
	if err != nil && (errors.IsNotFound(err) || errors.IsConflict(err)) {
		// We lost a race with something else changing the object
		log.Debugf("Ignoring error while deleting %s: %s", desc, err)
		err = nil
	}
	if err != nil {
		return fmt.Errorf("Error deleting %s: %s", desc, err)
	}

	return nil
}

func walkObjects(pool dynamic.ClientPool, disco discovery.DiscoveryInterface, core v1.CoreV1Interface, listMode string, listopts, nsListopts metav1.ListOptions, callback func(runtime.Object) error) error {
	rsrclists, err := disco.ServerResources()
	if err != nil {
		return err
	}

	namespaceList, err := core.Namespaces().List(nsListopts)
	if err != nil {
		return err
	}

	var namespaces []string
	switch listMode {
	case GcListModeClusterScope:
		namespaces = []string{metav1.NamespaceAll}
	case GcListModePerNamespace:
		for _, ns := range namespaceList.Items {
			namespaces = append(namespaces, ns.GetName())
		}
	default:
		return fmt.Errorf("unknown list mode %q", listMode)
	}

	// listResource lists all objects of a given resource.
	// If the list operation is forbidden for a given resource in a given namespace it returns skip true,
	// otherwise it invokes the callback for every found item.
	listResource := func(ns string, gvk schema.GroupVersionKind, rsrc metav1.APIResource) (skip bool, err error) {
		if !stringListContains(rsrc.Verbs, "list") {
			log.Debugf("Don't know how to list %v, skipping", rsrc)
			return false, nil
		}
		client, err := pool.ClientForGroupVersionKind(gvk)
		if err != nil {
			return false, err
		}

		rc := client.Resource(&rsrc, ns)
		log.Debugf("Listing %s", gvk)
		obj, err := rc.List(listopts)
		if err != nil {
			if errors.IsForbidden(err) {
				log.Warningf("Cannot list %s objects in namespace %q", gvk, ns)
				log.Debugf("Permission error: %v", err)
				return true, err
			}
			return false, err
		}
		if err := meta.EachListItem(obj, callback); err != nil {
			return false, err
		}
		return false, nil
	}

	for _, rsrclist := range rsrclists {
		gv, err := schema.ParseGroupVersion(rsrclist.GroupVersion)
		if err != nil {
			return err
		}
		for _, rsrc := range rsrclist.APIResources {
			gvk := gv.WithKind(rsrc.Kind)

			if rsrc.Namespaced {
				for _, ns := range namespaces {
					log.Debugf("Namespace %q", ns)
					if skip, err := listResource(ns, gvk, rsrc); err != nil && !skip {
						return err
					}
				}
			} else {
				if skip, err := listResource(metav1.NamespaceNone, gvk, rsrc); err != nil && !skip {
					return err
				}
			}
		}
	}
	return nil
}

func eligibleForGc(obj metav1.Object, gcTag string) bool {
	for _, ref := range obj.GetOwnerReferences() {
		if ref.Controller != nil && *ref.Controller {
			// Has a controller ref
			return false
		}
	}

	a := obj.GetAnnotations()

	strategy, ok := a[AnnotationGcStrategy]
	if !ok {
		strategy = GcStrategyAuto
	}

	return a[AnnotationGcTag] == gcTag &&
		strategy == GcStrategyAuto
}
