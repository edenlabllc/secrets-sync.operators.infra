package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	"secrets-sync.operators.infra/types"

	V1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

func (c *controller) iteratorDstNS(dstNamespaces []string, handlerSecret func(ns string) error) error {
	for _, ns := range dstNamespaces {
		if _, err := c.ClientSet.CoreV1().Namespaces().Get(context.TODO(), ns, metaV1.GetOptions{}); errors.IsNotFound(err) {
			klog.Infof("Namespace: %s is not created", ns)
			continue
		} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
			return fmt.Errorf("Error getting Namespace %v\n", statusError.ErrStatus.Message)
		} else if err != nil {
			return err
		}

		if err := handlerSecret(ns); err != nil {
			return err
		}
	}

	return nil
}

func (c *controller) deleteDstSecrets(key string) error {
	secretName := func(secret types.Secret, ns string) string {
		if _, ok := secret.Overwrite[ns]; ok {
			if len(secret.Overwrite[ns].DstSecretName) > 0 {
				return secret.Overwrite[ns].DstSecretName
			}
		}

		return secret.Name
	}

	for _, secret := range c.SecretList.Secrets {
		if secret.SrcNamespace+"/"+secret.Name == key {
			klog.Infof("Src secret %s does not exist anymore\n", key)
			if err := c.iteratorDstNS(
				secret.DstNamespaces,
				func(ns string) error {
					dstSecret, err := c.ClientSet.CoreV1().Secrets(ns).Get(context.TODO(), secretName(secret, ns), metaV1.GetOptions{})
					if errors.IsNotFound(err) {
						return nil
					} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
						return fmt.Errorf("Error getting Secret %v\n", statusError.ErrStatus.Message)
					} else if err != nil {
						return err
					}

					if err := c.ClientSet.CoreV1().Secrets(ns).Delete(context.TODO(), dstSecret.GetName(), metaV1.DeleteOptions{}); err != nil {
						return err
					}

					return nil
				},
			); err != nil {
				return err
			}
		}
	}

	return nil
}

func (c *controller) createUpdateDstSecrets(obj interface{}) error {
	var (
		keys     []string
		saveKeys []string
	)
	srcSecret := obj.(*V1.Secret)

	for key := range srcSecret.Data {
		keys = append(keys, key)
	}

	for _, secret := range c.SecretList.Secrets {
		if secret.Name == srcSecret.GetName() && secret.SrcNamespace == srcSecret.GetNamespace() {
			klog.Infof("Sync/Add for secret %s, namespace: %s\n", srcSecret.GetName(), srcSecret.GetNamespace())
			if err := c.iteratorDstNS(
				secret.DstNamespaces,
				func(ns string) error {
					newSecret := srcSecret.DeepCopy()

					if _, ok := secret.Overwrite[ns]; ok {
						if len(secret.Overwrite[ns].DstSecretName) > 0 {
							newSecret.SetName(secret.Overwrite[ns].DstSecretName)
						}

						if len(secret.Overwrite[ns].DstKeys) > 0 {
							for _, key := range keys {
								delete(newSecret.Data, key)
							}

							saveKeys = keys
							keys = []string{}

							for key := range srcSecret.Data {
								if _, ok := secret.Overwrite[ns].DstKeys[key]; ok {
									newSecret.Data[secret.Overwrite[ns].DstKeys[key]] = srcSecret.Data[key]
									keys = append(keys, key+"="+secret.Overwrite[ns].DstKeys[key])
								}
							}
						}
					}

					newSecret.SetNamespace(ns)
					newSecret.ObjectMeta.OwnerReferences = []metaV1.OwnerReference{}
					newSecret.Labels = map[string]string{}

					newSecret.Annotations = make(map[string]string)
					newSecret.Annotations[types.SyncAtAnnotation] = time.Now().Format(time.RFC3339)
					newSecret.Annotations[types.SyncFromSecretAnnotation] = srcSecret.Name
					newSecret.Annotations[types.SyncFromVersionAnnotation] = srcSecret.ResourceVersion
					newSecret.Annotations[types.SyncKeysAnnotation] = strings.Join(keys, ",")

					dstSecret, err := c.ClientSet.CoreV1().Secrets(ns).Get(context.TODO(),
						newSecret.GetName(), metaV1.GetOptions{})
					if errors.IsNotFound(err) {
						newSecret.ObjectMeta.ResourceVersion = ""
						if _, err := c.ClientSet.CoreV1().Secrets(ns).Create(context.TODO(),
							newSecret, metaV1.CreateOptions{}); err != nil {
							return err
						}

						klog.Infof("Created dst secret %s in dst namespace: %s",
							newSecret.GetName(), newSecret.GetNamespace())
					} else if statusError, isStatus := err.(*errors.StatusError); isStatus {
						return fmt.Errorf("Error getting Secrets %v\n", statusError.ErrStatus.Message)
					} else if err != nil {
						return err
					} else {
						if !reflect.DeepEqual(dstSecret.Data, newSecret.Data) {
							dstSecret.Annotations[types.SyncKeysAnnotation] = strings.Join(keys, ",")
							dstSecret.Data = newSecret.Data
							_, err := c.ClientSet.CoreV1().Secrets(ns).Update(context.TODO(),
								dstSecret, metaV1.UpdateOptions{})
							if err != nil {
								return err
							}

							klog.Infof("Update data dst secret %s in dst namespace: %s",
								newSecret.GetName(), newSecret.GetNamespace())
						}
					}

					keys = saveKeys
					return nil
				},
			); err != nil {
				return err
			}
		}
	}

	return nil
}
