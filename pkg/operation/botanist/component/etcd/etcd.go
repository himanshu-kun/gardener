// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package etcd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/semver"
	druidv1alpha1 "github.com/gardener/etcd-druid/api/v1alpha1"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2beta1 "k8s.io/api/autoscaling/v2beta1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/intstr"
	vpaautoscalingv1 "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controllerutils"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	"github.com/gardener/gardener/pkg/operation/botanist/component/monitoring"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
)

// Class is a string type alias for etcd classes.
type Class string

const (
	// ClassNormal is a constant for a normal etcd (without extensive metrics or higher resource settings, etc.)
	ClassNormal Class = "normal"
	// ClassImportant is a constant for an important etcd (with extensive metrics or higher resource settings, etc.).
	// Such etcds are also unsafe to evict (from the PoV of the cluster-autoscaler when trying to scale down).
	ClassImportant Class = "important"

	// SecretNameClient is the name of the secret containing the client certificate and key for the etcd.
	SecretNameClient       = "etcd-client"
	secretNamePrefixServer = "etcd-server-"

	// secretNamePrefixPeerServer is the prefix for the secret containing the server certificate and key for the etcd peer network.
	secretNamePrefixPeerServer = "etcd-peer-server-"

	// LabelAppValue is the value of a label whose key is 'app'.
	LabelAppValue = "etcd-statefulset"

	// NetworkPolicyNameClient is the name of a network policy that allows ingress traffic to etcd from certain sources.
	NetworkPolicyNameClient = "allow-etcd"

	// NetworkPolicyNamePeer is the name of a network policy that allows ingress traffic to etcd from member pods.
	NetworkPolicyNamePeer = "allow-etcd-peer"

	portNameClient        = "client"
	portNameBackupRestore = "backuprestore"

	statefulSetNamePrefix      = "etcd"
	containerNameEtcd          = "etcd"
	containerNameBackupRestore = "backup-restore"
)

var (
	// TimeNow is a function returning the current time exposed for testing.
	TimeNow = time.Now

	// PortEtcdPeer is the port exposed by etcd for server-to-server communication.
	PortEtcdPeer = int32(2380)
	// PortEtcdClient is the port exposed by etcd for client communication.
	PortEtcdClient = int32(2379)
	// PortBackupRestore is the client port exposed by the backup-restore sidecar container.
	PortBackupRestore = int32(8080)
)

// ServiceName returns the service name for an etcd for the given role.
func ServiceName(role string) string {
	return fmt.Sprintf("etcd-%s-client", role)
}

// Interface contains functions for a etcd deployer.
type Interface interface {
	component.DeployWaiter
	component.MonitoringComponent
	// Snapshot triggers the backup-restore sidecar to perform a full snapshot in case backup configuration is provided.
	Snapshot(context.Context, kubernetes.PodExecutor) error
	// SetBackupConfig sets the backup configuration.
	SetBackupConfig(config *BackupConfig)
	// SetHVPAConfig sets the HVPA configuration.
	SetHVPAConfig(config *HVPAConfig)
	// Get retrieves the Etcd resource
	Get(context.Context) (*druidv1alpha1.Etcd, error)
	// Scale scales the etcd resource to the given replica count.
	Scale(context.Context, int32) error
	// RolloutPeerCA gets the peer CA and patches the
	// related `etcd` resource to use this new CA for peer communication.
	RolloutPeerCA(context.Context) error
}

// New creates a new instance of DeployWaiter for the Etcd.
func New(
	log logr.Logger,
	c client.Client,
	namespace string,
	secretsManager secretsmanager.Interface,
	values Values,
) Interface {
	name := values.NamePrefix + "etcd-" + values.Role
	log = log.WithValues("etcd", client.ObjectKey{Namespace: namespace, Name: name})

	return &etcd{
		client:         c,
		log:            log,
		namespace:      namespace,
		secretsManager: secretsManager,
		values:         values,
		etcd: &druidv1alpha1.Etcd{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
		},
	}
}

type etcd struct {
	client         client.Client
	log            logr.Logger
	namespace      string
	secretsManager secretsmanager.Interface
	values         Values
	etcd           *druidv1alpha1.Etcd
}

// Values are the configuration values for the ETCD.
type Values struct {
	NamePrefix              string
	Role                    string
	Class                   Class
	Replicas                *int32
	StorageCapacity         string
	StorageClassName        *string
	DefragmentationSchedule *string
	CARotationPhase         gardencorev1beta1.CredentialsRotationPhase
	KubernetesVersion       *semver.Version
	BackupConfig            *BackupConfig
	HvpaConfig              *HVPAConfig
	PriorityClassName       string
}

func (e *etcd) Deploy(ctx context.Context) error {
	var (
		existingEtcd *druidv1alpha1.Etcd
		existingSts  *appsv1.StatefulSet
	)

	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), e.etcd); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		existingEtcd = e.etcd.DeepCopy()
	}

	stsName := e.etcd.Name
	if existingEtcd != nil && existingEtcd.Status.Etcd != nil && existingEtcd.Status.Etcd.Name != "" {
		stsName = existingEtcd.Status.Etcd.Name
	}

	var sts appsv1.StatefulSet
	if err := e.client.Get(ctx, client.ObjectKey{Namespace: e.namespace, Name: stsName}, &sts); err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
	} else {
		existingSts = &sts
	}

	var (
		clientNetworkPolicy = e.emptyNetworkPolicy(NetworkPolicyNameClient)
		peerNetworkPolicy   = e.emptyNetworkPolicy(NetworkPolicyNamePeer)
		hvpa                = e.emptyHVPA()

		replicas = e.computeReplicas(existingEtcd)

		protocolTCP             = corev1.ProtocolTCP
		intStrPortEtcdClient    = intstr.FromInt(int(PortEtcdClient))
		intStrPortEtcdPeer      = intstr.FromInt(int(PortEtcdPeer))
		intStrPortBackupRestore = intstr.FromInt(int(PortBackupRestore))

		resourcesEtcd, resourcesBackupRestore = e.computeContainerResources(existingSts)
		quota                                 = resource.MustParse("8Gi")
		storageCapacity                       = resource.MustParse(e.values.StorageCapacity)
		garbageCollectionPolicy               = druidv1alpha1.GarbageCollectionPolicy(druidv1alpha1.GarbageCollectionPolicyExponential)
		garbageCollectionPeriod               = metav1.Duration{Duration: 12 * time.Hour}
		compressionPolicy                     = druidv1alpha1.GzipCompression
		compressionSpec                       = druidv1alpha1.CompressionSpec{
			Enabled: pointer.Bool(true),
			Policy:  &compressionPolicy,
		}

		annotations         map[string]string
		metrics             = druidv1alpha1.Basic
		volumeClaimTemplate = e.etcd.Name
		minAllowed          = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("50m"),
			corev1.ResourceMemory: resource.MustParse("200M"),
		}
	)

	if e.values.Class == ClassImportant {
		annotations = map[string]string{"cluster-autoscaler.kubernetes.io/safe-to-evict": "false"}
		metrics = druidv1alpha1.Extensive
		volumeClaimTemplate = e.values.Role + "-" + strings.TrimSuffix(e.etcd.Name, "-"+e.values.Role)
		minAllowed = corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("200m"),
			corev1.ResourceMemory: resource.MustParse("700M"),
		}
	}

	etcdCASecret, found := e.secretsManager.Get(v1beta1constants.SecretNameCAETCD)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCD)
	}

	serverSecret, err := e.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNamePrefixServer + e.values.Role,
		CommonName:                  "etcd-server",
		DNSNames:                    e.clientServiceDNSNames(),
		CertType:                    secretsutils.ServerClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	clientSecret, err := e.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        SecretNameClient,
		CommonName:                  "etcd-client",
		CertType:                    secretsutils.ClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCD), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		return err
	}

	// add peer certs if shoot has HA control plane
	var (
		etcdPeerCASecretName string
		peerServerSecretName string
	)

	if etcdPeerCASecretName, peerServerSecretName, err = e.handlePeerCertificates(ctx); err != nil {
		return err
	}

	// Without this if condition, both `etcdMain` and `etcdEvents` component deployers execute this code. However, these
	// network policies are not specified to them (they apply to both because there is no `role` label in the selector).
	// Hence, it doesn't make sense if both component deployers are running this code - let's only do it for the main
	// ETCD.
	if e.values.Role == v1beta1constants.ETCDRoleMain {
		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, clientNetworkPolicy, func() error {
			clientNetworkPolicy.Annotations = map[string]string{
				v1beta1constants.GardenerDescription: "Allows Ingress to etcd pods from the Shoot's Kubernetes API Server.",
			}
			clientNetworkPolicy.Labels = map[string]string{
				v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
			}
			clientNetworkPolicy.Spec.PodSelector = metav1.LabelSelector{
				MatchLabels: GetLabels(),
			}
			clientNetworkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
				{
					From: []networkingv1.NetworkPolicyPeer{
						{
							PodSelector: &metav1.LabelSelector{
								// TODO: Replace below map with a function call to the to-be-introduced kubeapiserver package.
								MatchLabels: map[string]string{
									v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
									v1beta1constants.LabelApp:   v1beta1constants.LabelKubernetes,
									v1beta1constants.LabelRole:  v1beta1constants.LabelAPIServer,
								},
							},
						},
						{
							PodSelector: &metav1.LabelSelector{
								MatchLabels: monitoring.GetPrometheusLabels(),
							},
						},
					},
					Ports: []networkingv1.NetworkPolicyPort{
						{
							Protocol: &protocolTCP,
							Port:     &intStrPortEtcdClient,
						},
						{
							Protocol: &protocolTCP,
							Port:     &intStrPortBackupRestore,
						},
					},
				},
			}
			clientNetworkPolicy.Spec.Egress = nil
			clientNetworkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{networkingv1.PolicyTypeIngress}
			return nil
		}); err != nil {
			return err
		}

		// create peer network policy only if there are 3 replicas
		if pointer.Int32Deref(e.values.Replicas, 0) > 1 {
			if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, peerNetworkPolicy, func() error {
				peerNetworkPolicy.Annotations = map[string]string{
					v1beta1constants.GardenerDescription: "Allows Ingress to etcd pods from etcd pods for peer communication.",
				}
				peerNetworkPolicy.Labels = map[string]string{
					v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
				}
				peerNetworkPolicy.Spec.PodSelector = metav1.LabelSelector{
					MatchLabels: GetLabels(),
				}
				peerNetworkPolicy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{
					{
						Ports: []networkingv1.NetworkPolicyPort{
							{
								Protocol: &protocolTCP,
								Port:     &intStrPortEtcdClient,
							},
							{
								Protocol: &protocolTCP,
								Port:     &intStrPortBackupRestore,
							},
							{
								Protocol: &protocolTCP,
								Port:     &intStrPortEtcdPeer,
							},
						},
						To: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: GetLabels(),
								},
							},
						},
					},
				}
				peerNetworkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{
					{
						From: []networkingv1.NetworkPolicyPeer{
							{
								PodSelector: &metav1.LabelSelector{
									MatchLabels: GetLabels(),
								},
							},
						},
						Ports: []networkingv1.NetworkPolicyPort{
							{
								Protocol: &protocolTCP,
								Port:     &intStrPortEtcdClient,
							},
							{
								Protocol: &protocolTCP,
								Port:     &intStrPortBackupRestore,
							},
							{
								Protocol: &protocolTCP,
								Port:     &intStrPortEtcdPeer,
							},
						},
					},
				}
				peerNetworkPolicy.Spec.PolicyTypes = []networkingv1.PolicyType{
					networkingv1.PolicyTypeIngress,
					networkingv1.PolicyTypeEgress,
				}
				return nil
			}); err != nil {
				return err
			}
		}
	}

	if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, e.etcd, func() error {
		metav1.SetMetaDataAnnotation(&e.etcd.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationReconcile)
		metav1.SetMetaDataAnnotation(&e.etcd.ObjectMeta, v1beta1constants.GardenerTimestamp, TimeNow().UTC().String())

		e.etcd.Labels = map[string]string{
			v1beta1constants.LabelRole:  e.values.Role,
			v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		}
		e.etcd.Spec.Replicas = replicas
		e.etcd.Spec.PriorityClassName = &e.values.PriorityClassName
		e.etcd.Spec.Annotations = annotations
		e.etcd.Spec.Labels = utils.MergeStringMaps(e.getRoleLabels(), map[string]string{
			v1beta1constants.LabelApp:                            LabelAppValue,
			v1beta1constants.LabelNetworkPolicyToDNS:             v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPublicNetworks:  v1beta1constants.LabelNetworkPolicyAllowed,
			v1beta1constants.LabelNetworkPolicyToPrivateNetworks: v1beta1constants.LabelNetworkPolicyAllowed,
			// TODO(rfranzke): etcd-druid wrongly uses all these labels for the .spec.selector of the Services it
			//  creates. Hence, we cannot change them because otherwise multi-node ETCD clusters would break (since the
			//  services wouldn't select anything anymore). Until this is fixed, we have to keep using the deprecated
			//  to-seed-apiserver label instead of the new to-runtime-apiserver.
			v1beta1constants.LabelNetworkPolicyToSeedAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
			// v1beta1constants.LabelNetworkPolicyToRuntimeAPIServer: v1beta1constants.LabelNetworkPolicyAllowed,
		})
		e.etcd.Spec.Selector = &metav1.LabelSelector{
			MatchLabels: utils.MergeStringMaps(e.getRoleLabels(), map[string]string{
				v1beta1constants.LabelApp: LabelAppValue,
			}),
		}
		e.etcd.Spec.Etcd = druidv1alpha1.EtcdConfig{
			Resources: resourcesEtcd,
			ClientUrlTLS: &druidv1alpha1.TLSConfig{
				TLSCASecretRef: druidv1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      etcdCASecret.Name,
						Namespace: etcdCASecret.Namespace,
					},
					DataKey: pointer.String(secretsutils.DataKeyCertificateBundle),
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      serverSecret.Name,
					Namespace: serverSecret.Namespace,
				},
				ClientTLSSecretRef: corev1.SecretReference{
					Name:      clientSecret.Name,
					Namespace: clientSecret.Namespace,
				},
			},
			ServerPort:              &PortEtcdPeer,
			ClientPort:              &PortEtcdClient,
			Metrics:                 &metrics,
			DefragmentationSchedule: e.computeDefragmentationSchedule(existingEtcd),
			Quota:                   &quota,
		}

		// TODO(timuthy): Once https://github.com/gardener/etcd-backup-restore/issues/538 is resolved we can enable PeerUrlTLS for all remaining clusters as well.
		if pointer.Int32Deref(e.values.Replicas, 0) > 1 {
			e.etcd.Spec.Etcd.PeerUrlTLS = &druidv1alpha1.TLSConfig{
				TLSCASecretRef: druidv1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      etcdPeerCASecretName,
						Namespace: e.namespace,
					},
					DataKey: pointer.String(secretsutils.DataKeyCertificateBundle),
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      peerServerSecretName,
					Namespace: e.namespace,
				},
			}
		}

		e.etcd.Spec.Backup = druidv1alpha1.BackupSpec{
			TLS: &druidv1alpha1.TLSConfig{
				TLSCASecretRef: druidv1alpha1.SecretReference{
					SecretReference: corev1.SecretReference{
						Name:      etcdCASecret.Name,
						Namespace: etcdCASecret.Namespace,
					},
					DataKey: pointer.String(secretsutils.DataKeyCertificateBundle),
				},
				ServerTLSSecretRef: corev1.SecretReference{
					Name:      serverSecret.Name,
					Namespace: serverSecret.Namespace,
				},
				ClientTLSSecretRef: corev1.SecretReference{
					Name:      clientSecret.Name,
					Namespace: clientSecret.Namespace,
				},
			},
			Port:                    &PortBackupRestore,
			Resources:               resourcesBackupRestore,
			GarbageCollectionPolicy: &garbageCollectionPolicy,
			GarbageCollectionPeriod: &garbageCollectionPeriod,
			SnapshotCompression:     &compressionSpec,
		}

		if e.values.BackupConfig != nil {
			var (
				provider                 = druidv1alpha1.StorageProvider(e.values.BackupConfig.Provider)
				deltaSnapshotPeriod      = metav1.Duration{Duration: 5 * time.Minute}
				deltaSnapshotMemoryLimit = resource.MustParse("100Mi")
			)

			e.etcd.Spec.Backup.Store = &druidv1alpha1.StoreSpec{
				SecretRef: &corev1.SecretReference{Name: e.values.BackupConfig.SecretRefName},
				Container: &e.values.BackupConfig.Container,
				Provider:  &provider,
				Prefix:    fmt.Sprintf("%s/etcd-%s", e.values.BackupConfig.Prefix, e.values.Role),
			}
			e.etcd.Spec.Backup.FullSnapshotSchedule = e.computeFullSnapshotSchedule(existingEtcd)
			e.etcd.Spec.Backup.DeltaSnapshotPeriod = &deltaSnapshotPeriod
			e.etcd.Spec.Backup.DeltaSnapshotMemoryLimit = &deltaSnapshotMemoryLimit

			if e.values.BackupConfig.LeaderElection != nil {
				e.etcd.Spec.Backup.LeaderElection = &druidv1alpha1.LeaderElectionSpec{
					EtcdConnectionTimeout: e.values.BackupConfig.LeaderElection.EtcdConnectionTimeout,
					ReelectionPeriod:      e.values.BackupConfig.LeaderElection.ReelectionPeriod,
				}
			}
		}

		e.etcd.Spec.StorageCapacity = &storageCapacity
		e.etcd.Spec.StorageClass = e.values.StorageClassName
		e.etcd.Spec.VolumeClaimTemplate = &volumeClaimTemplate
		return nil
	}); err != nil {
		return err
	}

	if e.values.HvpaConfig != nil && e.values.HvpaConfig.Enabled {
		var (
			hpaLabels          = map[string]string{v1beta1constants.LabelRole: "etcd-hpa-" + e.values.Role}
			vpaLabels          = map[string]string{v1beta1constants.LabelRole: "etcd-vpa-" + e.values.Role}
			updateModeAuto     = hvpav1alpha1.UpdateModeAuto
			containerPolicyOff = vpaautoscalingv1.ContainerScalingModeOff
			controlledValues   = vpaautoscalingv1.ContainerControlledValuesRequestsOnly
		)

		scaleDownUpdateMode := e.values.HvpaConfig.ScaleDownUpdateMode
		if scaleDownUpdateMode == nil {
			scaleDownUpdateMode = pointer.String(hvpav1alpha1.UpdateModeMaintenanceWindow)
		}

		if _, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, hvpa, func() error {
			hvpa.Labels = utils.MergeStringMaps(e.getRoleLabels(), map[string]string{
				v1beta1constants.LabelApp: LabelAppValue,
			})
			hvpa.Spec.Replicas = pointer.Int32(1)
			hvpa.Spec.MaintenanceTimeWindow = &hvpav1alpha1.MaintenanceTimeWindow{
				Begin: e.values.HvpaConfig.MaintenanceTimeWindow.Begin,
				End:   e.values.HvpaConfig.MaintenanceTimeWindow.End,
			}
			hvpa.Spec.Hpa = hvpav1alpha1.HpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: hpaLabels},
				Deploy:   false,
				Template: hvpav1alpha1.HpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: hpaLabels,
					},
					Spec: hvpav1alpha1.HpaTemplateSpec{
						MinReplicas: pointer.Int32(replicas),
						MaxReplicas: replicas,
						Metrics: []autoscalingv2beta1.MetricSpec{
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceCPU,
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
							{
								Type: autoscalingv2beta1.ResourceMetricSourceType,
								Resource: &autoscalingv2beta1.ResourceMetricSource{
									Name:                     corev1.ResourceMemory,
									TargetAverageUtilization: pointer.Int32(80),
								},
							},
						},
					},
				},
			}
			hvpa.Spec.Vpa = hvpav1alpha1.VpaSpec{
				Selector: &metav1.LabelSelector{MatchLabels: vpaLabels},
				Deploy:   true,
				ScaleUp: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: &updateModeAuto,
					},
					StabilizationDuration: pointer.String("5m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("1"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("2G"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				ScaleDown: hvpav1alpha1.ScaleType{
					UpdatePolicy: hvpav1alpha1.UpdatePolicy{
						UpdateMode: scaleDownUpdateMode,
					},
					StabilizationDuration: pointer.String("15m"),
					MinChange: hvpav1alpha1.ScaleParams{
						CPU: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("1"),
							Percentage: pointer.Int32(80),
						},
						Memory: hvpav1alpha1.ChangeParams{
							Value:      pointer.String("2G"),
							Percentage: pointer.Int32(80),
						},
					},
				},
				LimitsRequestsGapScaleParams: hvpav1alpha1.ScaleParams{
					CPU: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("2"),
						Percentage: pointer.Int32(40),
					},
					Memory: hvpav1alpha1.ChangeParams{
						Value:      pointer.String("5G"),
						Percentage: pointer.Int32(40),
					},
				},
				Template: hvpav1alpha1.VpaTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: vpaLabels,
					},
					Spec: hvpav1alpha1.VpaTemplateSpec{
						ResourcePolicy: &vpaautoscalingv1.PodResourcePolicy{
							ContainerPolicies: []vpaautoscalingv1.ContainerResourcePolicy{
								{
									ContainerName: containerNameEtcd,
									MinAllowed:    minAllowed,
									MaxAllowed: corev1.ResourceList{
										corev1.ResourceCPU:    resource.MustParse("4"),
										corev1.ResourceMemory: resource.MustParse("30G"),
									},
									ControlledValues: &controlledValues,
								},
								{
									ContainerName:    containerNameBackupRestore,
									Mode:             &containerPolicyOff,
									ControlledValues: &controlledValues,
								},
							},
						},
					},
				},
			}
			hvpa.Spec.WeightBasedScalingIntervals = []hvpav1alpha1.WeightBasedScalingInterval{
				{
					VpaWeight:         hvpav1alpha1.VpaOnly,
					StartReplicaCount: int32(replicas),
					LastReplicaCount:  int32(replicas),
				},
			}
			hvpa.Spec.TargetRef = &autoscalingv2beta1.CrossVersionObjectReference{
				APIVersion: appsv1.SchemeGroupVersion.String(),
				Kind:       "StatefulSet",
				Name:       stsName,
			}
			return nil
		}); err != nil {
			return err
		}
	} else {
		if err := kubernetesutils.DeleteObjects(ctx, e.client, hvpa); err != nil {
			return err
		}
	}

	return nil
}

func (e *etcd) Destroy(ctx context.Context) error {
	if err := gardenerutils.ConfirmDeletion(ctx, e.client, e.etcd); client.IgnoreNotFound(err) != nil {
		return err
	}

	objects := []client.Object{
		e.emptyHVPA(),
		e.etcd,
		e.emptyNetworkPolicy(NetworkPolicyNameClient),
	}

	if pointer.Int32Deref(e.values.Replicas, 0) > 1 {
		objects = append(objects, e.emptyNetworkPolicy(NetworkPolicyNamePeer))
	}

	return kubernetesutils.DeleteObjects(ctx, e.client, objects...)
}

func (e *etcd) getRoleLabels() map[string]string {
	return utils.MergeStringMaps(map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelRole:  e.values.Role,
	})
}

// GetLabels returns a set of labels that is common for all etcd resources.
func GetLabels() map[string]string {
	return map[string]string{
		v1beta1constants.GardenRole: v1beta1constants.GardenRoleControlPlane,
		v1beta1constants.LabelApp:   LabelAppValue,
	}
}

func (e *etcd) emptyNetworkPolicy(name string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: e.namespace}}
}

func (e *etcd) emptyHVPA() *hvpav1alpha1.Hvpa {
	return &hvpav1alpha1.Hvpa{ObjectMeta: metav1.ObjectMeta{Name: e.etcd.Name, Namespace: e.namespace}}
}

func (e *etcd) Snapshot(ctx context.Context, podExecutor kubernetes.PodExecutor) error {
	if e.values.BackupConfig == nil {
		return fmt.Errorf("no backup is configured for this etcd, cannot make a snapshot")
	}

	etcdMainSelector := e.podLabelSelector()

	podsList := &corev1.PodList{}
	if err := e.client.List(ctx, podsList, client.InNamespace(e.namespace), client.MatchingLabelsSelector{Selector: etcdMainSelector}); err != nil {
		return err
	}
	if len(podsList.Items) == 0 {
		return fmt.Errorf("didn't find any pods for selector: %v", etcdMainSelector)
	}

	_, err := podExecutor.Execute(
		e.namespace,
		podsList.Items[0].GetName(),
		containerNameBackupRestore,
		"/bin/sh",
		fmt.Sprintf("curl -k https://etcd-%s-local:%d/snapshot/full?final=true", e.values.Role, PortBackupRestore),
	)
	return err
}

func (e *etcd) clientServiceDNSNames() []string {
	var domainNames []string
	domainNames = append(domainNames, fmt.Sprintf("%s-local", e.etcd.Name))
	domainNames = append(domainNames, kubernetesutils.DNSNamesForService(fmt.Sprintf("%s-client", e.etcd.Name), e.namespace)...)

	// The peer service needs to be considered here since the etcd-backup-restore side-car
	// connects to member pods via pod domain names (e.g. for defragmentation).
	// See https://github.com/gardener/etcd-backup-restore/issues/494
	domainNames = append(domainNames, kubernetesutils.DNSNamesForService(fmt.Sprintf("*.%s-peer", e.etcd.Name), e.namespace)...)

	return domainNames
}

func (e *etcd) peerServiceDNSNames() []string {
	return append(
		kubernetesutils.DNSNamesForService(fmt.Sprintf("%s-peer", e.etcd.Name), e.namespace),
		kubernetesutils.DNSNamesForService(fmt.Sprintf("*.%s-peer", e.etcd.Name), e.namespace)...,
	)
}

// Get retrieves the Etcd resource
func (e *etcd) Get(ctx context.Context) (*druidv1alpha1.Etcd, error) {
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), e.etcd); err != nil {
		return nil, err
	}
	return e.etcd, nil
}

func (e *etcd) SetBackupConfig(backupConfig *BackupConfig) { e.values.BackupConfig = backupConfig }
func (e *etcd) SetHVPAConfig(hvpaConfig *HVPAConfig)       { e.values.HvpaConfig = hvpaConfig }

func (e *etcd) Scale(ctx context.Context, replicas int32) error {
	etcdObj := &druidv1alpha1.Etcd{}
	if err := e.client.Get(ctx, client.ObjectKeyFromObject(e.etcd), etcdObj); err != nil {
		return err
	}

	if expectedTimestamp, ok := e.etcd.Annotations[v1beta1constants.GardenerTimestamp]; ok {
		if err := health.ObjectHasAnnotationWithValue(v1beta1constants.GardenerTimestamp, expectedTimestamp)(etcdObj); err != nil {
			return err
		}
	}

	if _, ok := etcdObj.Annotations[v1beta1constants.GardenerOperation]; ok {
		return fmt.Errorf("etcd object still has operation annotation set")
	}

	patch := client.MergeFrom(etcdObj.DeepCopy())
	if e.etcd.Annotations == nil {
		etcdObj.SetAnnotations(make(map[string]string))
	}

	etcdObj.Annotations[v1beta1constants.GardenerOperation] = v1beta1constants.GardenerOperationReconcile
	etcdObj.Annotations[v1beta1constants.GardenerTimestamp] = TimeNow().UTC().String()
	etcdObj.Spec.Replicas = replicas

	e.etcd = etcdObj

	return e.client.Patch(ctx, etcdObj, patch)
}

func (e *etcd) RolloutPeerCA(ctx context.Context) error {
	if pointer.Int32Deref(e.values.Replicas, 0) != 3 {
		return nil
	}

	etcdPeerCASecret, found := e.secretsManager.Get(v1beta1constants.SecretNameCAETCDPeer)
	if !found {
		return fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCDPeer)
	}

	_, err := controllerutils.GetAndCreateOrMergePatch(ctx, e.client, e.etcd, func() error {
		// Exit early if etcd object has already the expected CA reference.
		if peerTLS := e.etcd.Spec.Etcd.PeerUrlTLS; peerTLS != nil &&
			peerTLS.TLSCASecretRef.Name == etcdPeerCASecret.Name {
			return nil
		}

		e.etcd.Annotations = map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationReconcile,
			v1beta1constants.GardenerTimestamp: TimeNow().UTC().String(),
		}

		var dataKey *string
		if e.etcd.Spec.Etcd.PeerUrlTLS != nil {
			dataKey = e.etcd.Spec.Etcd.PeerUrlTLS.TLSCASecretRef.DataKey
		}

		if e.etcd.Spec.Etcd.PeerUrlTLS == nil {
			e.etcd.Spec.Etcd.PeerUrlTLS = &druidv1alpha1.TLSConfig{}
		}

		e.etcd.Spec.Etcd.PeerUrlTLS.TLSCASecretRef = druidv1alpha1.SecretReference{
			SecretReference: corev1.SecretReference{
				Name:      etcdPeerCASecret.Name,
				Namespace: e.etcd.Namespace,
			},
			DataKey: dataKey,
		}
		return nil
	})
	return err
}

func (e *etcd) podLabelSelector() labels.Selector {
	app, _ := labels.NewRequirement(v1beta1constants.LabelApp, selection.Equals, []string{LabelAppValue})
	role, _ := labels.NewRequirement(v1beta1constants.LabelRole, selection.Equals, []string{e.values.Role})
	return labels.NewSelector().Add(*role, *app)
}

func (e *etcd) computeContainerResources(existingSts *appsv1.StatefulSet) (*corev1.ResourceRequirements, *corev1.ResourceRequirements) {
	var (
		resourcesEtcd = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("300m"),
				corev1.ResourceMemory: resource.MustParse("1G"),
			},
		}
		resourcesBackupRestore = &corev1.ResourceRequirements{
			Requests: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("23m"),
				corev1.ResourceMemory: resource.MustParse("128Mi"),
			},
		}
	)

	if existingSts != nil && e.values.HvpaConfig != nil && e.values.HvpaConfig.Enabled {
		for k := range existingSts.Spec.Template.Spec.Containers {
			v := existingSts.Spec.Template.Spec.Containers[k]
			switch v.Name {
			case containerNameEtcd:
				resourcesEtcd = &corev1.ResourceRequirements{
					Requests: v.Resources.Requests,
				}
			case containerNameBackupRestore:
				resourcesBackupRestore = &corev1.ResourceRequirements{
					Requests: v.Resources.Requests,
				}
			}
		}
	}

	return resourcesEtcd, resourcesBackupRestore
}

func (e *etcd) computeReplicas(existingEtcd *druidv1alpha1.Etcd) int32 {
	if e.values.Replicas != nil {
		return *e.values.Replicas
	}

	if existingEtcd != nil {
		return existingEtcd.Spec.Replicas
	}
	return 0
}

func (e *etcd) computeDefragmentationSchedule(existingEtcd *druidv1alpha1.Etcd) *string {
	defragmentationSchedule := e.values.DefragmentationSchedule
	if existingEtcd != nil && existingEtcd.Spec.Etcd.DefragmentationSchedule != nil {
		defragmentationSchedule = existingEtcd.Spec.Etcd.DefragmentationSchedule
	}
	return defragmentationSchedule
}

func (e *etcd) computeFullSnapshotSchedule(existingEtcd *druidv1alpha1.Etcd) *string {
	fullSnapshotSchedule := &e.values.BackupConfig.FullSnapshotSchedule
	if existingEtcd != nil && existingEtcd.Spec.Backup.FullSnapshotSchedule != nil {
		fullSnapshotSchedule = existingEtcd.Spec.Backup.FullSnapshotSchedule
	}
	return fullSnapshotSchedule
}

func (e *etcd) handlePeerCertificates(ctx context.Context) (caSecretName, peerSecretName string, err error) {
	// TODO(timuthy): Remove this once https://github.com/gardener/etcd-backup-restore/issues/538 is resolved.
	if pointer.Int32Deref(e.values.Replicas, 0) != 3 {
		return
	}

	etcdPeerCASecret, found := e.secretsManager.Get(v1beta1constants.SecretNameCAETCDPeer)
	if !found {
		err = fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCDPeer)
		return
	}

	var signedByCAOptions []secretsmanager.SignedByCAOption
	if e.values.CARotationPhase == gardencorev1beta1.RotationPreparing {
		signedByCAOptions = append(signedByCAOptions, secretsmanager.UseCurrentCA)
	}

	peerServerSecret, err := e.secretsManager.Generate(ctx, &secretsutils.CertificateSecretConfig{
		Name:                        secretNamePrefixPeerServer + e.values.Role,
		CommonName:                  "etcd-server",
		DNSNames:                    e.peerServiceDNSNames(),
		CertType:                    secretsutils.ServerClientCert,
		SkipPublishingCACertificate: true,
	}, secretsmanager.SignedByCA(v1beta1constants.SecretNameCAETCDPeer, signedByCAOptions...), secretsmanager.Rotate(secretsmanager.InPlace))
	if err != nil {
		err = fmt.Errorf("secret %q not found", v1beta1constants.SecretNameCAETCDPeer)
		return
	}

	caSecretName = etcdPeerCASecret.Name
	peerSecretName = peerServerSecret.Name
	return
}

// BackupConfig contains information for configuring the backup-restore sidecar so that it takes regularly backups of
// the etcd's data directory.
type BackupConfig struct {
	// Provider is the name of the infrastructure provider for the blob storage bucket.
	Provider string
	// Container is the name of the blob storage bucket.
	Container string
	// SecretRefName is the name of a Secret object containing the credentials of the selected infrastructure provider.
	SecretRefName string
	// Prefix is a prefix that shall be used for the filename of the backups of this etcd.
	Prefix string
	// FullSnapshotSchedule is a cron schedule that declares how frequent full snapshots shall be taken.
	FullSnapshotSchedule string
	// LeaderElection contains configuration for the leader election for the etcd backup-restore sidecar.
	LeaderElection *gardenletconfig.ETCDBackupLeaderElection
}

// HVPAConfig contains information for configuring the HVPA object for the etcd.
type HVPAConfig struct {
	// Enabled states whether an HVPA object shall be deployed.
	Enabled bool
	// MaintenanceTimeWindow contains begin and end of a time window that allows down-scaling the etcd in case its
	// resource requests/limits are unnecessarily high.
	MaintenanceTimeWindow gardencorev1beta1.MaintenanceTimeWindow
	// The update mode to use for scale down.
	ScaleDownUpdateMode *string
}
