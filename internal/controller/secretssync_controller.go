/*
Copyright 2023 @apanasiuk-el edenlabllc.
*/

package controller

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/labels"
	"reflect"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	internalv1alpha1 "secrets-sync.operators.infra/api/v1alpha1"
)

const (
	frequency = time.Second * 3
	ownerKind = "internal.edenlab.io/owner-kind"
	ownerName = "internal.edenlab.io/owner-name"
)

var (
	secretMeta = metav1.TypeMeta{
		APIVersion: "v1",
		Kind:       "Secret",
	}
)

// SecretsSyncReconciler reconciles a SecretsSync object
type SecretsSyncReconciler struct {
	*SystemInfo
	Scheme *runtime.Scheme
	client.Client
}

type SystemInfo struct {
	ctx         context.Context
	req         ctrl.Request
	reqLogger   logr.Logger
	secretsSync *internalv1alpha1.SecretsSync
}

//+kubebuilder:rbac:groups=internal.edenlab.io,resources=secretssyncs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=internal.edenlab.io,resources=secretssyncs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=internal.edenlab.io,resources=secretssyncs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SecretsSync object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.4/pkg/reconcile
func (r *SecretsSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var newSecrets []*v1.Secret

	r.ctx = ctx
	r.req = req
	r.reqLogger = log.FromContext(ctx)
	r.secretsSync = &internalv1alpha1.SecretsSync{}

	if err := r.Client.Get(r.ctx, req.NamespacedName, r.secretsSync); err != nil {
		if errors.IsNotFound(err) {
			r.reqLogger.Error(nil, fmt.Sprintf("Can not find CRD by name: %s", r.req.Name))
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	for srcSecretName, val := range r.secretsSync.Spec.Secrets {
		if err := r.Client.Get(r.ctx, types.NamespacedName{Name: val.SrcNamespace}, &v1.Namespace{}); err != nil {
			if errors.IsNotFound(err) {
				r.reqLogger.Error(err, fmt.Sprintf("Source namespace for secret %s not exists", srcSecretName))
				r.updateStatusCRD("Failed",
					fmt.Sprintf("Source namespace for secret %s not exists", srcSecretName), 0)
				continue
			} else {
				return ctrl.Result{}, err
			}
		} else {
			srcSecret := &v1.Secret{}
			if err := r.Client.Get(r.ctx, types.NamespacedName{Name: srcSecretName, Namespace: val.SrcNamespace}, srcSecret); err != nil {
				if errors.IsNotFound(err) {
					r.reqLogger.Error(err, fmt.Sprintf("Source secret %s not exists in namespace %s",
						srcSecretName, val.SrcNamespace))
					r.updateStatusCRD("Failed",
						fmt.Sprintf("Source secret %s not exists in namespace %s", srcSecretName, val.SrcNamespace),
						0)
					continue
				} else {
					return ctrl.Result{}, err
				}
			} else {
				newSecrets = append(newSecrets, r.GenerateSecrets(val.DstSecrets, srcSecret)...)
			}
		}
	}

	//r.reqLogger.Info(fmt.Sprintf("List of %d copied secrets has been generated", len(newSecrets)))

	if err := r.garbageCollector(newSecrets...); err != nil {
		return ctrl.Result{}, err
	}

	//fmt.Printf("%+v\n", newSecrets)
	defSecret := &v1.Secret{}
	for _, secret := range newSecrets {
		if err := r.Client.Get(r.ctx, types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, defSecret); err != nil {
			if errors.IsNotFound(err) {
				if err := r.CreateSecret(secret); err != nil {
					return ctrl.Result{}, err
				}

				r.reqLogger.Info(fmt.Sprintf("New secret %s has been created for namespace %s", secret.Name, secret.Namespace))
				r.updateStatusCRD("Created", "", len(newSecrets))
				continue
			} else {
				return ctrl.Result{}, err
			}
		}

		if !reflect.DeepEqual(defSecret.Data, secret.Data) {
			if err := r.Client.Delete(r.ctx, secret); err != nil {
				return ctrl.Result{}, err
			}

			if err := r.CreateSecret(secret); err != nil {
				return ctrl.Result{}, err
			}

			r.reqLogger.Info(fmt.Sprintf("Secret %s has been updated due to a difference in the data field", secret.Name))
			r.updateStatusCRD("Updated", "", len(newSecrets))
		}
	}

	return ctrl.Result{RequeueAfter: frequency}, nil
}

func (r *SecretsSyncReconciler) updateStatusCRD(phase, errorCRD string, count int) {
	if count > 0 {
		r.secretsSync.Status.CreatedTime = &metav1.Time{Time: time.Now()}
		r.secretsSync.Status.Count = count
	}

	r.secretsSync.Status.Phase = phase
	r.secretsSync.Status.Error = errorCRD
	if err := r.Status().Update(r.ctx, r.secretsSync); err != nil {
		r.reqLogger.Error(err, fmt.Sprintf("Unable to update status for CRD: %s", r.req.Name))
	} else {
		r.reqLogger.Info(fmt.Sprintf("Update status for CRD: %s", r.req.Name))
	}
}

func (r *SecretsSyncReconciler) GenerateSecrets(dstSecrets []internalv1alpha1.DstSecret, srcSecret *v1.Secret) []*v1.Secret {
	var (
		newSecrets   []*v1.Secret
		secretLabels = map[string]string{
			ownerKind: "SecretsSync",
			ownerName: r.req.Name,
		}
		secretName string
	)

	if len(dstSecrets) > 0 {
		for _, dstSecret := range dstSecrets {
			if len(dstSecret.Name) > 0 {
				secretName = dstSecret.Name
			} else {
				secretName = srcSecret.Name
			}

			newSecret := &v1.Secret{
				TypeMeta: secretMeta,
				ObjectMeta: metav1.ObjectMeta{
					Labels:    secretLabels,
					Name:      secretName,
					Namespace: r.req.Namespace,
				},
				Type: srcSecret.Type,
			}

			data := make(map[string][]byte)
			for key, val := range srcSecret.Data {
				if keyName, ok := dstSecret.Keys[key]; ok {
					data[keyName] = val
				} else {
					data[key] = val
				}
			}

			stringData := make(map[string]string)
			for key, val := range srcSecret.StringData {
				if keyName, ok := dstSecret.Keys[key]; ok {
					stringData[keyName] = val
				} else {
					stringData[key] = val
				}
			}

			newSecret.Data = data
			newSecret.StringData = stringData
			newSecrets = append(newSecrets, newSecret)
		}

		return newSecrets
	} else {
		return append(newSecrets, &v1.Secret{
			TypeMeta: secretMeta,
			ObjectMeta: metav1.ObjectMeta{
				Labels:    secretLabels,
				Name:      srcSecret.Name,
				Namespace: r.req.Namespace,
			},
			Data:       srcSecret.Data,
			StringData: srcSecret.StringData,
			Type:       srcSecret.Type,
		})
	}
}

func (r *SecretsSyncReconciler) CreateSecret(secret *v1.Secret) error {
	// Used to ensure that the secret will be deleted when the custom resource object is removed
	if err := ctrl.SetControllerReference(r.secretsSync, secret, r.Scheme); err != nil {
		return err
	}

	if err := r.Client.Create(r.ctx, secret); err != nil {
		r.updateStatusCRD("Failed", err.Error(), 0)
		return err
	}

	return nil
}

func (r *SecretsSyncReconciler) garbageCollector(secrets ...*v1.Secret) error {
	var (
		secretLabels = map[string]string{
			ownerKind: "SecretsSync",
			ownerName: r.req.Name,
		}
	)

	listSecrets := &v1.SecretList{}
	listOps := &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(secretLabels),
		Namespace:     r.req.Namespace,
	}

	if err := r.Client.List(r.ctx, listSecrets, listOps); err != nil {
		return err
	}

	deleteListSecret := make(map[string]*v1.Secret, len(listSecrets.Items))
	for _, secret := range secrets {
		deleteListSecret[secret.Name] = secret
	}

	for _, item := range listSecrets.Items {
		if _, ok := deleteListSecret[item.Name]; !ok {
			if err := r.Client.Delete(r.ctx, &item); err != nil {
				return err
			}

			r.reqLogger.Info(fmt.Sprintf("Secret removed %s", item.Name))
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SecretsSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&internalv1alpha1.SecretsSync{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		Complete(r)
}
