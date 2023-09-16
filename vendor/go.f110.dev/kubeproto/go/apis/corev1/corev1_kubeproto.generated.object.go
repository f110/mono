package corev1

import (
	"go.f110.dev/kubeproto/go/apis/metav1"
	apiresource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilintstr "k8s.io/apimachinery/pkg/util/intstr"
)

const GroupName = ""

var (
	GroupVersion       = metav1.GroupVersion{Group: GroupName, Version: "v1"}
	SchemeBuilder      = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme        = SchemeBuilder.AddToScheme
	SchemaGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}
)

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemaGroupVersion,
		&Binding{},
		&BindingList{},
		&ComponentStatus{},
		&ComponentStatusList{},
		&ConfigMap{},
		&ConfigMapList{},
		&Endpoints{},
		&EndpointsList{},
		&Event{},
		&EventList{},
		&LimitRange{},
		&LimitRangeList{},
		&Namespace{},
		&NamespaceList{},
		&Node{},
		&NodeList{},
		&PersistentVolume{},
		&PersistentVolumeClaim{},
		&PersistentVolumeClaimList{},
		&PersistentVolumeList{},
		&Pod{},
		&PodList{},
		&PodStatusResult{},
		&PodStatusResultList{},
		&PodTemplate{},
		&PodTemplateList{},
		&RangeAllocation{},
		&RangeAllocationList{},
		&ReplicationController{},
		&ReplicationControllerList{},
		&ResourceQuota{},
		&ResourceQuotaList{},
		&Secret{},
		&SecretList{},
		&Service{},
		&ServiceAccount{},
		&ServiceAccountList{},
		&ServiceList{},
	)
	metav1.AddToGroupVersion(scheme, SchemaGroupVersion)
	return nil
}

type AzureDataDiskCachingMode string

const (
	AzureDataDiskCachingModeNone      AzureDataDiskCachingMode = "None"
	AzureDataDiskCachingModeReadOnly  AzureDataDiskCachingMode = "ReadOnly"
	AzureDataDiskCachingModeReadWrite AzureDataDiskCachingMode = "ReadWrite"
)

type AzureDataDiskKind string

const (
	AzureDataDiskKindShared    AzureDataDiskKind = "Shared"
	AzureDataDiskKindDedicated AzureDataDiskKind = "Dedicated"
	AzureDataDiskKindManaged   AzureDataDiskKind = "Managed"
)

type ComponentConditionType string

const (
	ComponentConditionTypeHealthy ComponentConditionType = "Healthy"
)

type ConditionStatus string

const (
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"
	ConditionStatusUnknown ConditionStatus = "Unknown"
)

type DNSPolicy string

const (
	DNSPolicyClusterFirstWithHostNet DNSPolicy = "ClusterFirstWithHostNet"
	DNSPolicyClusterFirst            DNSPolicy = "ClusterFirst"
	DNSPolicyDefault                 DNSPolicy = "Default"
	DNSPolicyNone                    DNSPolicy = "None"
)

type FinalizerName string

const (
	FinalizerNameKubernetes FinalizerName = "kubernetes"
)

type HostPathType string

const (
	HostPathTypeHostPathUnset     HostPathType = "HostPathUnset"
	HostPathTypeDirectoryOrCreate HostPathType = "DirectoryOrCreate"
	HostPathTypeDirectory         HostPathType = "Directory"
	HostPathTypeFileOrCreate      HostPathType = "FileOrCreate"
	HostPathTypeFile              HostPathType = "File"
	HostPathTypeSocket            HostPathType = "Socket"
	HostPathTypeCharDevice        HostPathType = "CharDevice"
	HostPathTypeBlockDevice       HostPathType = "BlockDevice"
)

type IPFamily string

const (
	IPFamilyIpv4 IPFamily = "IPv4"
	IPFamilyIpv6 IPFamily = "IPv6"
)

type IPFamilyPolicy string

const (
	IPFamilyPolicySingleStack      IPFamilyPolicy = "SingleStack"
	IPFamilyPolicyPreferDualStack  IPFamilyPolicy = "PreferDualStack"
	IPFamilyPolicyRequireDualStack IPFamilyPolicy = "RequireDualStack"
)

type LimitType string

const (
	LimitTypePod                   LimitType = "Pod"
	LimitTypeContainer             LimitType = "Container"
	LimitTypePersistentVolumeClaim LimitType = "PersistentVolumeClaim"
)

type MountPropagationMode string

const (
	MountPropagationModeNone            MountPropagationMode = "None"
	MountPropagationModeHostToContainer MountPropagationMode = "HostToContainer"
	MountPropagationModeBidirectional   MountPropagationMode = "Bidirectional"
)

type NamespaceConditionType string

const (
	NamespaceConditionTypeNamespaceDeletionDiscoveryFailure           NamespaceConditionType = "NamespaceDeletionDiscoveryFailure"
	NamespaceConditionTypeNamespaceDeletionContentFailure             NamespaceConditionType = "NamespaceDeletionContentFailure"
	NamespaceConditionTypeNamespaceDeletionGroupVersionParsingFailure NamespaceConditionType = "NamespaceDeletionGroupVersionParsingFailure"
	NamespaceConditionTypeNamespaceContentRemaining                   NamespaceConditionType = "NamespaceContentRemaining"
	NamespaceConditionTypeNamespaceFinalizersRemaining                NamespaceConditionType = "NamespaceFinalizersRemaining"
)

type NamespacePhase string

const (
	NamespacePhaseActive      NamespacePhase = "Active"
	NamespacePhaseTerminating NamespacePhase = "Terminating"
)

type NodeAddressType string

const (
	NodeAddressTypeHostname    NodeAddressType = "Hostname"
	NodeAddressTypeInternalIP  NodeAddressType = "InternalIP"
	NodeAddressTypeExternalIP  NodeAddressType = "ExternalIP"
	NodeAddressTypeInternalDNS NodeAddressType = "InternalDNS"
	NodeAddressTypeExternalDNS NodeAddressType = "ExternalDNS"
)

type NodeConditionType string

const (
	NodeConditionTypeReady              NodeConditionType = "Ready"
	NodeConditionTypeMemoryPressure     NodeConditionType = "MemoryPressure"
	NodeConditionTypeDiskPressure       NodeConditionType = "DiskPressure"
	NodeConditionTypePIDPressure        NodeConditionType = "PIDPressure"
	NodeConditionTypeNetworkUnavailable NodeConditionType = "NetworkUnavailable"
)

type NodeInclusionPolicy string

const (
	NodeInclusionPolicyIgnore NodeInclusionPolicy = "Ignore"
	NodeInclusionPolicyHonor  NodeInclusionPolicy = "Honor"
)

type NodePhase string

const (
	NodePhasePending    NodePhase = "Pending"
	NodePhaseRunning    NodePhase = "Running"
	NodePhaseTerminated NodePhase = "Terminated"
)

type NodeSelectorOperator string

const (
	NodeSelectorOperatorIn           NodeSelectorOperator = "In"
	NodeSelectorOperatorNotIn        NodeSelectorOperator = "NotIn"
	NodeSelectorOperatorExists       NodeSelectorOperator = "Exists"
	NodeSelectorOperatorDoesNotExist NodeSelectorOperator = "DoesNotExist"
	NodeSelectorOperatorGt           NodeSelectorOperator = "Gt"
	NodeSelectorOperatorLt           NodeSelectorOperator = "Lt"
)

type OSName string

const (
	OSNameLinux   OSName = "linux"
	OSNameWindows OSName = "windows"
)

type PersistentVolumeAccessMode string

const (
	PersistentVolumeAccessModeReadWriteOnce    PersistentVolumeAccessMode = "ReadWriteOnce"
	PersistentVolumeAccessModeReadOnlyMany     PersistentVolumeAccessMode = "ReadOnlyMany"
	PersistentVolumeAccessModeReadWriteMany    PersistentVolumeAccessMode = "ReadWriteMany"
	PersistentVolumeAccessModeReadWriteOncePod PersistentVolumeAccessMode = "ReadWriteOncePod"
)

type PersistentVolumeClaimConditionType string

const (
	PersistentVolumeClaimConditionTypeResizing                PersistentVolumeClaimConditionType = "Resizing"
	PersistentVolumeClaimConditionTypeFileSystemResizePending PersistentVolumeClaimConditionType = "FileSystemResizePending"
)

type PersistentVolumeClaimPhase string

const (
	PersistentVolumeClaimPhasePending PersistentVolumeClaimPhase = "Pending"
	PersistentVolumeClaimPhaseBound   PersistentVolumeClaimPhase = "Bound"
	PersistentVolumeClaimPhaseLost    PersistentVolumeClaimPhase = "Lost"
)

type PersistentVolumeClaimResizeStatus string

const (
	PersistentVolumeClaimResizeStatusPersistentVolumeClaimNoExpansionInProgress PersistentVolumeClaimResizeStatus = "PersistentVolumeClaimNoExpansionInProgress"
	PersistentVolumeClaimResizeStatusControllerExpansionInProgress              PersistentVolumeClaimResizeStatus = "ControllerExpansionInProgress"
	PersistentVolumeClaimResizeStatusControllerExpansionFailed                  PersistentVolumeClaimResizeStatus = "ControllerExpansionFailed"
	PersistentVolumeClaimResizeStatusNodeExpansionPending                       PersistentVolumeClaimResizeStatus = "NodeExpansionPending"
	PersistentVolumeClaimResizeStatusNodeExpansionInProgress                    PersistentVolumeClaimResizeStatus = "NodeExpansionInProgress"
	PersistentVolumeClaimResizeStatusNodeExpansionFailed                        PersistentVolumeClaimResizeStatus = "NodeExpansionFailed"
)

type PersistentVolumeMode string

const (
	PersistentVolumeModeBlock      PersistentVolumeMode = "Block"
	PersistentVolumeModeFilesystem PersistentVolumeMode = "Filesystem"
)

type PersistentVolumePhase string

const (
	PersistentVolumePhasePending   PersistentVolumePhase = "Pending"
	PersistentVolumePhaseAvailable PersistentVolumePhase = "Available"
	PersistentVolumePhaseBound     PersistentVolumePhase = "Bound"
	PersistentVolumePhaseReleased  PersistentVolumePhase = "Released"
	PersistentVolumePhaseFailed    PersistentVolumePhase = "Failed"
)

type PersistentVolumeReclaimPolicy string

const (
	PersistentVolumeReclaimPolicyRecycle PersistentVolumeReclaimPolicy = "Recycle"
	PersistentVolumeReclaimPolicyDelete  PersistentVolumeReclaimPolicy = "Delete"
	PersistentVolumeReclaimPolicyRetain  PersistentVolumeReclaimPolicy = "Retain"
)

type PodConditionType string

const (
	PodConditionTypeContainersReady  PodConditionType = "ContainersReady"
	PodConditionTypeInitialized      PodConditionType = "Initialized"
	PodConditionTypeReady            PodConditionType = "Ready"
	PodConditionTypePodScheduled     PodConditionType = "PodScheduled"
	PodConditionTypeDisruptionTarget PodConditionType = "DisruptionTarget"
)

type PodFSGroupChangePolicy string

const (
	PodFSGroupChangePolicyOnRootMismatch PodFSGroupChangePolicy = "OnRootMismatch"
	PodFSGroupChangePolicyAlways         PodFSGroupChangePolicy = "Always"
)

type PodPhase string

const (
	PodPhasePending   PodPhase = "Pending"
	PodPhaseRunning   PodPhase = "Running"
	PodPhaseSucceeded PodPhase = "Succeeded"
	PodPhaseFailed    PodPhase = "Failed"
	PodPhaseUnknown   PodPhase = "Unknown"
)

type PodQOSClass string

const (
	PodQOSClassGuaranteed PodQOSClass = "Guaranteed"
	PodQOSClassBurstable  PodQOSClass = "Burstable"
	PodQOSClassBestEffort PodQOSClass = "BestEffort"
)

type PodResizeStatus string

const (
	PodResizeStatusProposed   PodResizeStatus = "Proposed"
	PodResizeStatusInProgress PodResizeStatus = "InProgress"
	PodResizeStatusDeferred   PodResizeStatus = "Deferred"
	PodResizeStatusInfeasible PodResizeStatus = "Infeasible"
)

type PreemptionPolicy string

const (
	PreemptionPolicyPreemptLowerPriority PreemptionPolicy = "PreemptLowerPriority"
	PreemptionPolicyNever                PreemptionPolicy = "Never"
)

type ProcMountType string

const (
	ProcMountTypeDefault  ProcMountType = "Default"
	ProcMountTypeUnmasked ProcMountType = "Unmasked"
)

type Protocol string

const (
	ProtocolTCP  Protocol = "TCP"
	ProtocolUDP  Protocol = "UDP"
	ProtocolSCTP Protocol = "SCTP"
)

type PullPolicy string

const (
	PullPolicyAlways       PullPolicy = "Always"
	PullPolicyNever        PullPolicy = "Never"
	PullPolicyIfNotPresent PullPolicy = "IfNotPresent"
)

type ReplicationControllerConditionType string

const (
	ReplicationControllerConditionTypeReplicaFailure ReplicationControllerConditionType = "ReplicaFailure"
)

type ResourceName string

const (
	ResourceNameCpu                      ResourceName = "cpu"
	ResourceNameMemory                   ResourceName = "memory"
	ResourceNameStorage                  ResourceName = "storage"
	ResourceNameEphemeralStorage         ResourceName = "ephemeral-storage"
	ResourceNamePods                     ResourceName = "pods"
	ResourceNameServices                 ResourceName = "services"
	ResourceNameReplicationcontrollers   ResourceName = "replicationcontrollers"
	ResourceNameResourcequotas           ResourceName = "resourcequotas"
	ResourceNameSecrets                  ResourceName = "secrets"
	ResourceNameConfigmaps               ResourceName = "configmaps"
	ResourceNamePersistentvolumeclaims   ResourceName = "persistentvolumeclaims"
	ResourceNameServicesNodeports        ResourceName = "services.nodeports"
	ResourceNameServicesLoadbalancers    ResourceName = "services.loadbalancers"
	ResourceNameRequestsCpu              ResourceName = "requests.cpu"
	ResourceNameRequestsMemory           ResourceName = "requests.memory"
	ResourceNameRequestsStorage          ResourceName = "requests.storage"
	ResourceNameRequestsEphemeralStorage ResourceName = "requests.ephemeral-storage"
	ResourceNameLimitsCpu                ResourceName = "limits.cpu"
	ResourceNameLimitsMemory             ResourceName = "limits.memory"
	ResourceNameLimitsEphemeralStorage   ResourceName = "limits.ephemeral-storage"
)

type ResourceQuotaScope string

const (
	ResourceQuotaScopeTerminating               ResourceQuotaScope = "Terminating"
	ResourceQuotaScopeNotTerminating            ResourceQuotaScope = "NotTerminating"
	ResourceQuotaScopeBestEffort                ResourceQuotaScope = "BestEffort"
	ResourceQuotaScopeNotBestEffort             ResourceQuotaScope = "NotBestEffort"
	ResourceQuotaScopePriorityClass             ResourceQuotaScope = "PriorityClass"
	ResourceQuotaScopeCrossNamespacePodAffinity ResourceQuotaScope = "CrossNamespacePodAffinity"
)

type ResourceResizeRestartPolicy string

const (
	ResourceResizeRestartPolicyNotRequired      ResourceResizeRestartPolicy = "NotRequired"
	ResourceResizeRestartPolicyRestartContainer ResourceResizeRestartPolicy = "RestartContainer"
)

type RestartPolicy string

const (
	RestartPolicyAlways    RestartPolicy = "Always"
	RestartPolicyOnFailure RestartPolicy = "OnFailure"
	RestartPolicyNever     RestartPolicy = "Never"
)

type ScopeSelectorOperator string

const (
	ScopeSelectorOperatorIn           ScopeSelectorOperator = "In"
	ScopeSelectorOperatorNotIn        ScopeSelectorOperator = "NotIn"
	ScopeSelectorOperatorExists       ScopeSelectorOperator = "Exists"
	ScopeSelectorOperatorDoesNotExist ScopeSelectorOperator = "DoesNotExist"
)

type SeccompProfileType string

const (
	SeccompProfileTypeUnconfined     SeccompProfileType = "Unconfined"
	SeccompProfileTypeRuntimeDefault SeccompProfileType = "RuntimeDefault"
	SeccompProfileTypeLocalhost      SeccompProfileType = "Localhost"
)

type SecretType string

const (
	SecretTypeOpaque                          SecretType = "Opaque"
	SecretTypeKubernetesIoServiceAccountToken SecretType = "kubernetes.io/service-account-token"
	SecretTypeKubernetesIoDockercfg           SecretType = "kubernetes.io/dockercfg"
	SecretTypeKubernetesIoDockerconfigjson    SecretType = "kubernetes.io/dockerconfigjson"
	SecretTypeKubernetesIoBasicAuth           SecretType = "kubernetes.io/basic-auth"
	SecretTypeKubernetesIoSshAuth             SecretType = "kubernetes.io/ssh-auth"
	SecretTypeKubernetesIoTls                 SecretType = "kubernetes.io/tls"
	SecretTypeBootstrapKubernetesIoToken      SecretType = "bootstrap.kubernetes.io/token"
)

type ServiceAffinity string

const (
	ServiceAffinityClientIP ServiceAffinity = "ClientIP"
	ServiceAffinityNone     ServiceAffinity = "None"
)

type ServiceExternalTrafficPolicy string

const (
	ServiceExternalTrafficPolicyCluster ServiceExternalTrafficPolicy = "Cluster"
	ServiceExternalTrafficPolicyLocal   ServiceExternalTrafficPolicy = "Local"
)

type ServiceInternalTrafficPolicy string

const (
	ServiceInternalTrafficPolicyCluster ServiceInternalTrafficPolicy = "Cluster"
	ServiceInternalTrafficPolicyLocal   ServiceInternalTrafficPolicy = "Local"
)

type ServiceType string

const (
	ServiceTypeClusterIP    ServiceType = "ClusterIP"
	ServiceTypeNodePort     ServiceType = "NodePort"
	ServiceTypeLoadBalancer ServiceType = "LoadBalancer"
	ServiceTypeExternalName ServiceType = "ExternalName"
)

type StorageMedium string

const (
	StorageMediumDEFAULT   StorageMedium = "DEFAULT"
	StorageMediumMemory    StorageMedium = "Memory"
	StorageMediumHugePages StorageMedium = "HugePages"
)

type TaintEffect string

const (
	TaintEffectNoSchedule       TaintEffect = "NoSchedule"
	TaintEffectPreferNoSchedule TaintEffect = "PreferNoSchedule"
	TaintEffectNoExecute        TaintEffect = "NoExecute"
)

type TerminationMessagePolicy string

const (
	TerminationMessagePolicyFile                  TerminationMessagePolicy = "File"
	TerminationMessagePolicyFallbackToLogsOnError TerminationMessagePolicy = "FallbackToLogsOnError"
)

type TolerationOperator string

const (
	TolerationOperatorExists TolerationOperator = "Exists"
	TolerationOperatorEqual  TolerationOperator = "Equal"
)

type URIScheme string

const (
	URISchemeHTTP  URIScheme = "HTTP"
	URISchemeHTTPS URIScheme = "HTTPS"
)

type UnsatisfiableConstraintAction string

const (
	UnsatisfiableConstraintActionDoNotSchedule  UnsatisfiableConstraintAction = "DoNotSchedule"
	UnsatisfiableConstraintActionScheduleAnyway UnsatisfiableConstraintAction = "ScheduleAnyway"
)

type Binding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// The target object that you want to bind to the standard object.
	Target ObjectReference `json:"target"`
}

func (in *Binding) DeepCopyInto(out *Binding) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Target.DeepCopyInto(&out.Target)
}

func (in *Binding) DeepCopy() *Binding {
	if in == nil {
		return nil
	}
	out := new(Binding)
	in.DeepCopyInto(out)
	return out
}

func (in *Binding) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type BindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Binding `json:"items"`
}

func (in *BindingList) DeepCopyInto(out *BindingList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Binding, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *BindingList) DeepCopy() *BindingList {
	if in == nil {
		return nil
	}
	out := new(BindingList)
	in.DeepCopyInto(out)
	return out
}

func (in *BindingList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ComponentStatus struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// List of component conditions observed
	Conditions []ComponentCondition `json:"conditions"`
}

func (in *ComponentStatus) DeepCopyInto(out *ComponentStatus) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Conditions != nil {
		l := make([]ComponentCondition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
}

func (in *ComponentStatus) DeepCopy() *ComponentStatus {
	if in == nil {
		return nil
	}
	out := new(ComponentStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *ComponentStatus) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ComponentStatusList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ComponentStatus `json:"items"`
}

func (in *ComponentStatusList) DeepCopyInto(out *ComponentStatusList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]ComponentStatus, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ComponentStatusList) DeepCopy() *ComponentStatusList {
	if in == nil {
		return nil
	}
	out := new(ComponentStatusList)
	in.DeepCopyInto(out)
	return out
}

func (in *ComponentStatusList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ConfigMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Immutable, if set to true, ensures that data stored in the ConfigMap cannot
	// be updated (only object metadata can be modified).
	// If not set to true, the field can be modified at any time.
	// Defaulted to nil.
	Immutable bool `json:"immutable,omitempty"`
	// Data contains the configuration data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// Values with non-UTF-8 byte sequences must use the BinaryData field.
	// The keys stored in Data must not overlap with the keys in
	// the BinaryData field, this is enforced during validation process.
	Data map[string]string `json:"data,omitempty"`
	// BinaryData contains the binary data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// BinaryData can contain byte sequences that are not in the UTF-8 range.
	// The keys stored in BinaryData must not overlap with the ones in
	// the Data field, this is enforced during validation process.
	// Using this field will require 1.10+ apiserver and
	// kubelet.
	BinaryData map[string][]byte `json:"binaryData,omitempty"`
}

func (in *ConfigMap) DeepCopyInto(out *ConfigMap) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Data != nil {
		in, out := &in.Data, &out.Data
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.BinaryData != nil {
		in, out := &in.BinaryData, &out.BinaryData
		*out = make(map[string][]byte, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *ConfigMap) DeepCopy() *ConfigMap {
	if in == nil {
		return nil
	}
	out := new(ConfigMap)
	in.DeepCopyInto(out)
	return out
}

func (in *ConfigMap) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ConfigMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ConfigMap `json:"items"`
}

func (in *ConfigMapList) DeepCopyInto(out *ConfigMapList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]ConfigMap, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ConfigMapList) DeepCopy() *ConfigMapList {
	if in == nil {
		return nil
	}
	out := new(ConfigMapList)
	in.DeepCopyInto(out)
	return out
}

func (in *ConfigMapList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Endpoints struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// The set of all endpoints is the union of all subsets. Addresses are placed into
	// subsets according to the IPs they share. A single address with multiple ports,
	// some of which are ready and some of which are not (because they come from
	// different containers) will result in the address being displayed in different
	// subsets for the different ports. No address will appear in both Addresses and
	// NotReadyAddresses in the same subset.
	// Sets of addresses and ports that comprise a service.
	Subsets []EndpointSubset `json:"subsets"`
}

func (in *Endpoints) DeepCopyInto(out *Endpoints) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Subsets != nil {
		l := make([]EndpointSubset, len(in.Subsets))
		for i := range in.Subsets {
			in.Subsets[i].DeepCopyInto(&l[i])
		}
		out.Subsets = l
	}
}

func (in *Endpoints) DeepCopy() *Endpoints {
	if in == nil {
		return nil
	}
	out := new(Endpoints)
	in.DeepCopyInto(out)
	return out
}

func (in *Endpoints) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type EndpointsList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Endpoints `json:"items"`
}

func (in *EndpointsList) DeepCopyInto(out *EndpointsList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Endpoints, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *EndpointsList) DeepCopy() *EndpointsList {
	if in == nil {
		return nil
	}
	out := new(EndpointsList)
	in.DeepCopyInto(out)
	return out
}

func (in *EndpointsList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Event struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// The object that this event is about.
	InvolvedObject ObjectReference `json:"involvedObject"`
	// This should be a short, machine understandable string that gives the reason
	// for the transition into the object's current status.
	Reason string `json:"reason,omitempty"`
	// A human-readable description of the status of this operation.
	Message string `json:"message,omitempty"`
	// The component reporting this event. Should be a short machine understandable string.
	Source *EventSource `json:"source,omitempty"`
	// The time at which the event was first recorded. (Time of server receipt is in TypeMeta.)
	FirstTimestamp *metav1.Time `json:"firstTimestamp,omitempty"`
	// The time at which the most recent occurrence of this event was recorded.
	LastTimestamp *metav1.Time `json:"lastTimestamp,omitempty"`
	// The number of times this event has occurred.
	Count int `json:"count,omitempty"`
	// Type of this event (Normal, Warning), new types could be added in the future
	Type string `json:"type,omitempty"`
	// Time when this Event was first observed.
	EventTime *metav1.MicroTime `json:"eventTime,omitempty"`
	// Data about the Event series this event represents or nil if it's a singleton Event.
	Series *EventSeries `json:"series,omitempty"`
	// What action was taken/failed regarding to the Regarding object.
	Action string `json:"action,omitempty"`
	// Optional secondary object for more complex actions.
	Related *ObjectReference `json:"related,omitempty"`
	// Name of the controller that emitted this Event, e.g. `kubernetes.io/kubelet`.
	ReportingController string `json:"reportingComponent"`
	// ID of the controller instance, e.g. `kubelet-xyzf`.
	ReportingInstance string `json:"reportingInstance"`
}

func (in *Event) DeepCopyInto(out *Event) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.InvolvedObject.DeepCopyInto(&out.InvolvedObject)
	if in.Source != nil {
		in, out := &in.Source, &out.Source
		*out = new(EventSource)
		(*in).DeepCopyInto(*out)
	}
	if in.FirstTimestamp != nil {
		in, out := &in.FirstTimestamp, &out.FirstTimestamp
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.LastTimestamp != nil {
		in, out := &in.LastTimestamp, &out.LastTimestamp
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.EventTime != nil {
		in, out := &in.EventTime, &out.EventTime
		*out = new(metav1.MicroTime)
		(*in).DeepCopyInto(*out)
	}
	if in.Series != nil {
		in, out := &in.Series, &out.Series
		*out = new(EventSeries)
		(*in).DeepCopyInto(*out)
	}
	if in.Related != nil {
		in, out := &in.Related, &out.Related
		*out = new(ObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Event) DeepCopy() *Event {
	if in == nil {
		return nil
	}
	out := new(Event)
	in.DeepCopyInto(out)
	return out
}

func (in *Event) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type EventList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Event `json:"items"`
}

func (in *EventList) DeepCopyInto(out *EventList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Event, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *EventList) DeepCopy() *EventList {
	if in == nil {
		return nil
	}
	out := new(EventList)
	in.DeepCopyInto(out)
	return out
}

func (in *EventList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type LimitRange struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec defines the limits enforced.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *LimitRangeSpec `json:"spec,omitempty"`
}

func (in *LimitRange) DeepCopyInto(out *LimitRange) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(LimitRangeSpec)
		(*in).DeepCopyInto(*out)
	}
}

func (in *LimitRange) DeepCopy() *LimitRange {
	if in == nil {
		return nil
	}
	out := new(LimitRange)
	in.DeepCopyInto(out)
	return out
}

func (in *LimitRange) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type LimitRangeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []LimitRange `json:"items"`
}

func (in *LimitRangeList) DeepCopyInto(out *LimitRangeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]LimitRange, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *LimitRangeList) DeepCopy() *LimitRangeList {
	if in == nil {
		return nil
	}
	out := new(LimitRangeList)
	in.DeepCopyInto(out)
	return out
}

func (in *LimitRangeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Namespace struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec defines the behavior of the Namespace.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *NamespaceSpec `json:"spec,omitempty"`
	// Status describes the current status of a Namespace.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *NamespaceStatus `json:"status,omitempty"`
}

func (in *Namespace) DeepCopyInto(out *Namespace) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(NamespaceSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(NamespaceStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Namespace) DeepCopy() *Namespace {
	if in == nil {
		return nil
	}
	out := new(Namespace)
	in.DeepCopyInto(out)
	return out
}

func (in *Namespace) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type NamespaceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Namespace `json:"items"`
}

func (in *NamespaceList) DeepCopyInto(out *NamespaceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Namespace, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *NamespaceList) DeepCopy() *NamespaceList {
	if in == nil {
		return nil
	}
	out := new(NamespaceList)
	in.DeepCopyInto(out)
	return out
}

func (in *NamespaceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Node struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec defines the behavior of a node.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *NodeSpec `json:"spec,omitempty"`
	// Most recently observed status of the node.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *NodeStatus `json:"status,omitempty"`
}

func (in *Node) DeepCopyInto(out *Node) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(NodeSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(NodeStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Node) DeepCopy() *Node {
	if in == nil {
		return nil
	}
	out := new(Node)
	in.DeepCopyInto(out)
	return out
}

func (in *Node) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type NodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Node `json:"items"`
}

func (in *NodeList) DeepCopyInto(out *NodeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Node, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *NodeList) DeepCopy() *NodeList {
	if in == nil {
		return nil
	}
	out := new(NodeList)
	in.DeepCopyInto(out)
	return out
}

func (in *NodeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PersistentVolume struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// spec defines a specification of a persistent volume owned by the cluster.
	// Provisioned by an administrator.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistent-volumes
	Spec *PersistentVolumeSpec `json:"spec,omitempty"`
	// status represents the current information/status for the persistent volume.
	// Populated by the system.
	// Read-only.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistent-volumes
	Status *PersistentVolumeStatus `json:"status,omitempty"`
}

func (in *PersistentVolume) DeepCopyInto(out *PersistentVolume) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(PersistentVolumeSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(PersistentVolumeStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentVolume) DeepCopy() *PersistentVolume {
	if in == nil {
		return nil
	}
	out := new(PersistentVolume)
	in.DeepCopyInto(out)
	return out
}

func (in *PersistentVolume) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PersistentVolumeClaim struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// spec defines the desired characteristics of a volume requested by a pod author.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	Spec *PersistentVolumeClaimSpec `json:"spec,omitempty"`
	// status represents the current information/status of a persistent volume claim.
	// Read-only.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	Status *PersistentVolumeClaimStatus `json:"status,omitempty"`
}

func (in *PersistentVolumeClaim) DeepCopyInto(out *PersistentVolumeClaim) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(PersistentVolumeClaimSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(PersistentVolumeClaimStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentVolumeClaim) DeepCopy() *PersistentVolumeClaim {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaim)
	in.DeepCopyInto(out)
	return out
}

func (in *PersistentVolumeClaim) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PersistentVolumeClaimList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PersistentVolumeClaim `json:"items"`
}

func (in *PersistentVolumeClaimList) DeepCopyInto(out *PersistentVolumeClaimList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]PersistentVolumeClaim, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *PersistentVolumeClaimList) DeepCopy() *PersistentVolumeClaimList {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimList)
	in.DeepCopyInto(out)
	return out
}

func (in *PersistentVolumeClaimList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PersistentVolumeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PersistentVolume `json:"items"`
}

func (in *PersistentVolumeList) DeepCopyInto(out *PersistentVolumeList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]PersistentVolume, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *PersistentVolumeList) DeepCopy() *PersistentVolumeList {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeList)
	in.DeepCopyInto(out)
	return out
}

func (in *PersistentVolumeList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Pod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *PodSpec `json:"spec,omitempty"`
	// Most recently observed status of the pod.
	// This data may not be up to date.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *PodStatus `json:"status,omitempty"`
}

func (in *Pod) DeepCopyInto(out *Pod) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(PodSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(PodStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Pod) DeepCopy() *Pod {
	if in == nil {
		return nil
	}
	out := new(Pod)
	in.DeepCopyInto(out)
	return out
}

func (in *Pod) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PodList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Pod `json:"items"`
}

func (in *PodList) DeepCopyInto(out *PodList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Pod, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *PodList) DeepCopy() *PodList {
	if in == nil {
		return nil
	}
	out := new(PodList)
	in.DeepCopyInto(out)
	return out
}

func (in *PodList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PodStatusResult struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Most recently observed status of the pod.
	// This data may not be up to date.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *PodStatus `json:"status,omitempty"`
}

func (in *PodStatusResult) DeepCopyInto(out *PodStatusResult) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(PodStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodStatusResult) DeepCopy() *PodStatusResult {
	if in == nil {
		return nil
	}
	out := new(PodStatusResult)
	in.DeepCopyInto(out)
	return out
}

func (in *PodStatusResult) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PodStatusResultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PodStatusResult `json:"items"`
}

func (in *PodStatusResultList) DeepCopyInto(out *PodStatusResultList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]PodStatusResult, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *PodStatusResultList) DeepCopy() *PodStatusResultList {
	if in == nil {
		return nil
	}
	out := new(PodStatusResultList)
	in.DeepCopyInto(out)
	return out
}

func (in *PodStatusResultList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PodTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Template defines the pods that will be created from this pod template.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Template *PodTemplateSpec `json:"template,omitempty"`
}

func (in *PodTemplate) DeepCopyInto(out *PodTemplate) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Template != nil {
		in, out := &in.Template, &out.Template
		*out = new(PodTemplateSpec)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodTemplate) DeepCopy() *PodTemplate {
	if in == nil {
		return nil
	}
	out := new(PodTemplate)
	in.DeepCopyInto(out)
	return out
}

func (in *PodTemplate) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type PodTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []PodTemplate `json:"items"`
}

func (in *PodTemplateList) DeepCopyInto(out *PodTemplateList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]PodTemplate, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *PodTemplateList) DeepCopy() *PodTemplateList {
	if in == nil {
		return nil
	}
	out := new(PodTemplateList)
	in.DeepCopyInto(out)
	return out
}

func (in *PodTemplateList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type RangeAllocation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Range is string that identifies the range represented by 'data'.
	Range string `json:"range"`
	// Data is a bit array containing all allocated addresses in the previous segment.
	Data []byte `json:"data,omitempty"`
}

func (in *RangeAllocation) DeepCopyInto(out *RangeAllocation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
}

func (in *RangeAllocation) DeepCopy() *RangeAllocation {
	if in == nil {
		return nil
	}
	out := new(RangeAllocation)
	in.DeepCopyInto(out)
	return out
}

func (in *RangeAllocation) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type RangeAllocationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RangeAllocation `json:"items"`
}

func (in *RangeAllocationList) DeepCopyInto(out *RangeAllocationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]RangeAllocation, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *RangeAllocationList) DeepCopy() *RangeAllocationList {
	if in == nil {
		return nil
	}
	out := new(RangeAllocationList)
	in.DeepCopyInto(out)
	return out
}

func (in *RangeAllocationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ReplicationController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec defines the specification of the desired behavior of the replication controller.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *ReplicationControllerSpec `json:"spec,omitempty"`
	// Status is the most recently observed status of the replication controller.
	// This data may be out of date by some window of time.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *ReplicationControllerStatus `json:"status,omitempty"`
}

func (in *ReplicationController) DeepCopyInto(out *ReplicationController) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(ReplicationControllerSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(ReplicationControllerStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ReplicationController) DeepCopy() *ReplicationController {
	if in == nil {
		return nil
	}
	out := new(ReplicationController)
	in.DeepCopyInto(out)
	return out
}

func (in *ReplicationController) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ReplicationControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ReplicationController `json:"items"`
}

func (in *ReplicationControllerList) DeepCopyInto(out *ReplicationControllerList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]ReplicationController, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ReplicationControllerList) DeepCopy() *ReplicationControllerList {
	if in == nil {
		return nil
	}
	out := new(ReplicationControllerList)
	in.DeepCopyInto(out)
	return out
}

func (in *ReplicationControllerList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ResourceQuota struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec defines the desired quota.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *ResourceQuotaSpec `json:"spec,omitempty"`
	// Status defines the actual enforced quota and its current usage.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *ResourceQuotaStatus `json:"status,omitempty"`
}

func (in *ResourceQuota) DeepCopyInto(out *ResourceQuota) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(ResourceQuotaSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(ResourceQuotaStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ResourceQuota) DeepCopy() *ResourceQuota {
	if in == nil {
		return nil
	}
	out := new(ResourceQuota)
	in.DeepCopyInto(out)
	return out
}

func (in *ResourceQuota) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ResourceQuotaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ResourceQuota `json:"items"`
}

func (in *ResourceQuotaList) DeepCopyInto(out *ResourceQuotaList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]ResourceQuota, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ResourceQuotaList) DeepCopy() *ResourceQuotaList {
	if in == nil {
		return nil
	}
	out := new(ResourceQuotaList)
	in.DeepCopyInto(out)
	return out
}

func (in *ResourceQuotaList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Secret struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Immutable, if set to true, ensures that data stored in the Secret cannot
	// be updated (only object metadata can be modified).
	// If not set to true, the field can be modified at any time.
	// Defaulted to nil.
	Immutable bool `json:"immutable,omitempty"`
	// Data contains the secret data. Each key must consist of alphanumeric
	// characters, '-', '_' or '.'. The serialized form of the secret data is a
	// base64 encoded string, representing the arbitrary (possibly non-string)
	// data value here. Described in https://tools.ietf.org/html/rfc4648#section-4
	Data map[string][]byte `json:"data,omitempty"`
	// stringData allows specifying non-binary secret data in string form.
	// It is provided as a write-only input field for convenience.
	// All keys and values are merged into the data field on write, overwriting any existing values.
	// The stringData field is never output when reading from the API.
	StringData map[string]string `json:"stringData,omitempty"`
	// Used to facilitate programmatic handling of secret data.
	// More info: https://kubernetes.io/docs/concepts/configuration/secret/#secret-types
	Type SecretType `json:"type,omitempty"`
}

func (in *Secret) DeepCopyInto(out *Secret) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Data != nil {
		in, out := &in.Data, &out.Data
		*out = make(map[string][]byte, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.StringData != nil {
		in, out := &in.StringData, &out.StringData
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *Secret) DeepCopy() *Secret {
	if in == nil {
		return nil
	}
	out := new(Secret)
	in.DeepCopyInto(out)
	return out
}

func (in *Secret) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type SecretList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Secret `json:"items"`
}

func (in *SecretList) DeepCopyInto(out *SecretList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Secret, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *SecretList) DeepCopy() *SecretList {
	if in == nil {
		return nil
	}
	out := new(SecretList)
	in.DeepCopyInto(out)
	return out
}

func (in *SecretList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type Service struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Spec defines the behavior of a service.
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *ServiceSpec `json:"spec,omitempty"`
	// Most recently observed status of the service.
	// Populated by the system.
	// Read-only.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Status *ServiceStatus `json:"status,omitempty"`
}

func (in *Service) DeepCopyInto(out *Service) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(ServiceSpec)
		(*in).DeepCopyInto(*out)
	}
	if in.Status != nil {
		in, out := &in.Status, &out.Status
		*out = new(ServiceStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Service) DeepCopy() *Service {
	if in == nil {
		return nil
	}
	out := new(Service)
	in.DeepCopyInto(out)
	return out
}

func (in *Service) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ServiceAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	// Secrets is a list of the secrets in the same namespace that pods running using this ServiceAccount are allowed to use.
	// Pods are only limited to this list if this service account has a "kubernetes.io/enforce-mountable-secrets" annotation set to "true".
	// This field should not be used to find auto-generated service account token secrets for use outside of pods.
	// Instead, tokens can be requested directly using the TokenRequest API, or service account token secrets can be manually created.
	// More info: https://kubernetes.io/docs/concepts/configuration/secret
	Secrets []ObjectReference `json:"secrets"`
	// ImagePullSecrets is a list of references to secrets in the same namespace to use for pulling any images
	// in pods that reference this ServiceAccount. ImagePullSecrets are distinct from Secrets because Secrets
	// can be mounted in the pod, but ImagePullSecrets are only accessed by the kubelet.
	// More info: https://kubernetes.io/docs/concepts/containers/images/#specifying-imagepullsecrets-on-a-pod
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets"`
	// AutomountServiceAccountToken indicates whether pods running as this service account should have an API token automatically mounted.
	// Can be overridden at the pod level.
	AutomountServiceAccountToken bool `json:"automountServiceAccountToken,omitempty"`
}

func (in *ServiceAccount) DeepCopyInto(out *ServiceAccount) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	if in.Secrets != nil {
		l := make([]ObjectReference, len(in.Secrets))
		for i := range in.Secrets {
			in.Secrets[i].DeepCopyInto(&l[i])
		}
		out.Secrets = l
	}
	if in.ImagePullSecrets != nil {
		l := make([]LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			in.ImagePullSecrets[i].DeepCopyInto(&l[i])
		}
		out.ImagePullSecrets = l
	}
}

func (in *ServiceAccount) DeepCopy() *ServiceAccount {
	if in == nil {
		return nil
	}
	out := new(ServiceAccount)
	in.DeepCopyInto(out)
	return out
}

func (in *ServiceAccount) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ServiceAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ServiceAccount `json:"items"`
}

func (in *ServiceAccountList) DeepCopyInto(out *ServiceAccountList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]ServiceAccount, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ServiceAccountList) DeepCopy() *ServiceAccountList {
	if in == nil {
		return nil
	}
	out := new(ServiceAccountList)
	in.DeepCopyInto(out)
	return out
}

func (in *ServiceAccountList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Service `json:"items"`
}

func (in *ServiceList) DeepCopyInto(out *ServiceList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]Service, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ServiceList) DeepCopy() *ServiceList {
	if in == nil {
		return nil
	}
	out := new(ServiceList)
	in.DeepCopyInto(out)
	return out
}

func (in *ServiceList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type ObjectReference struct {
	// Kind of the referent.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind,omitempty"`
	// Namespace of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/namespaces/
	Namespace string `json:"namespace,omitempty"`
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name,omitempty"`
	// UID of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#uids
	UID string `json:"uid,omitempty"`
	// API version of the referent.
	APIVersion string `json:"apiVersion,omitempty"`
	// Specific resourceVersion to which this reference is made, if any.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#concurrency-control-and-consistency
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// If referring to a piece of an object instead of an entire object, this string
	// should contain a valid JSON/Go field access statement, such as desiredState.manifest.containers[2].
	// For example, if the object reference is to a container within a pod, this would take on a value like:
	// "spec.containers{name}" (where "name" refers to the name of the container that triggered
	// the event) or if no container name is specified "spec.containers[2]" (container with
	// index 2 in this pod). This syntax is chosen only to have some well-defined way of
	// referencing a part of an object.
	FieldPath string `json:"fieldPath,omitempty"`
}

func (in *ObjectReference) DeepCopyInto(out *ObjectReference) {
	*out = *in
}

func (in *ObjectReference) DeepCopy() *ObjectReference {
	if in == nil {
		return nil
	}
	out := new(ObjectReference)
	in.DeepCopyInto(out)
	return out
}

type ComponentCondition struct {
	// Type of condition for a component.
	// Valid value: "Healthy"
	Type ComponentConditionType `json:"type"`
	// Status of the condition for a component.
	// Valid values for "Healthy": "True", "False", or "Unknown".
	Status ConditionStatus `json:"status"`
	// Message about the condition for a component.
	// For example, information about a health check.
	Message string `json:"message,omitempty"`
	// Condition error code for a component.
	// For example, a health check error code.
	Error string `json:"error,omitempty"`
}

func (in *ComponentCondition) DeepCopyInto(out *ComponentCondition) {
	*out = *in
}

func (in *ComponentCondition) DeepCopy() *ComponentCondition {
	if in == nil {
		return nil
	}
	out := new(ComponentCondition)
	in.DeepCopyInto(out)
	return out
}

type EndpointSubset struct {
	// IP addresses which offer the related ports that are marked as ready. These endpoints
	// should be considered safe for load balancers and clients to utilize.
	Addresses []EndpointAddress `json:"addresses"`
	// IP addresses which offer the related ports but are not currently marked as ready
	// because they have not yet finished starting, have recently failed a readiness check,
	// or have recently failed a liveness check.
	NotReadyAddresses []EndpointAddress `json:"notReadyAddresses"`
	// Port numbers available on the related IP addresses.
	Ports []EndpointPort `json:"ports"`
}

func (in *EndpointSubset) DeepCopyInto(out *EndpointSubset) {
	*out = *in
	if in.Addresses != nil {
		l := make([]EndpointAddress, len(in.Addresses))
		for i := range in.Addresses {
			in.Addresses[i].DeepCopyInto(&l[i])
		}
		out.Addresses = l
	}
	if in.NotReadyAddresses != nil {
		l := make([]EndpointAddress, len(in.NotReadyAddresses))
		for i := range in.NotReadyAddresses {
			in.NotReadyAddresses[i].DeepCopyInto(&l[i])
		}
		out.NotReadyAddresses = l
	}
	if in.Ports != nil {
		l := make([]EndpointPort, len(in.Ports))
		for i := range in.Ports {
			in.Ports[i].DeepCopyInto(&l[i])
		}
		out.Ports = l
	}
}

func (in *EndpointSubset) DeepCopy() *EndpointSubset {
	if in == nil {
		return nil
	}
	out := new(EndpointSubset)
	in.DeepCopyInto(out)
	return out
}

type EventSource struct {
	// Component from which the event is generated.
	Component string `json:"component,omitempty"`
	// Node name on which the event is generated.
	Host string `json:"host,omitempty"`
}

func (in *EventSource) DeepCopyInto(out *EventSource) {
	*out = *in
}

func (in *EventSource) DeepCopy() *EventSource {
	if in == nil {
		return nil
	}
	out := new(EventSource)
	in.DeepCopyInto(out)
	return out
}

type EventSeries struct {
	// Number of occurrences in this series up to the last heartbeat time
	Count int `json:"count,omitempty"`
	// Time of the last occurrence observed
	LastObservedTime *metav1.MicroTime `json:"lastObservedTime,omitempty"`
}

func (in *EventSeries) DeepCopyInto(out *EventSeries) {
	*out = *in
	if in.LastObservedTime != nil {
		in, out := &in.LastObservedTime, &out.LastObservedTime
		*out = new(metav1.MicroTime)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EventSeries) DeepCopy() *EventSeries {
	if in == nil {
		return nil
	}
	out := new(EventSeries)
	in.DeepCopyInto(out)
	return out
}

type LimitRangeSpec struct {
	// Limits is the list of LimitRangeItem objects that are enforced.
	Limits []LimitRangeItem `json:"limits"`
}

func (in *LimitRangeSpec) DeepCopyInto(out *LimitRangeSpec) {
	*out = *in
	if in.Limits != nil {
		l := make([]LimitRangeItem, len(in.Limits))
		for i := range in.Limits {
			in.Limits[i].DeepCopyInto(&l[i])
		}
		out.Limits = l
	}
}

func (in *LimitRangeSpec) DeepCopy() *LimitRangeSpec {
	if in == nil {
		return nil
	}
	out := new(LimitRangeSpec)
	in.DeepCopyInto(out)
	return out
}

type NamespaceSpec struct {
	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/namespaces/
	Finalizers []FinalizerName `json:"finalizers"`
}

func (in *NamespaceSpec) DeepCopyInto(out *NamespaceSpec) {
	*out = *in
	if in.Finalizers != nil {
		t := make([]FinalizerName, len(in.Finalizers))
		copy(t, in.Finalizers)
		out.Finalizers = t
	}
}

func (in *NamespaceSpec) DeepCopy() *NamespaceSpec {
	if in == nil {
		return nil
	}
	out := new(NamespaceSpec)
	in.DeepCopyInto(out)
	return out
}

type NamespaceStatus struct {
	// Phase is the current lifecycle phase of the namespace.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/namespaces/
	Phase NamespacePhase `json:"phase,omitempty"`
	// Represents the latest available observations of a namespace's current state.
	Conditions []NamespaceCondition `json:"conditions"`
}

func (in *NamespaceStatus) DeepCopyInto(out *NamespaceStatus) {
	*out = *in
	if in.Conditions != nil {
		l := make([]NamespaceCondition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
}

func (in *NamespaceStatus) DeepCopy() *NamespaceStatus {
	if in == nil {
		return nil
	}
	out := new(NamespaceStatus)
	in.DeepCopyInto(out)
	return out
}

type NodeSpec struct {
	// PodCIDR represents the pod IP range assigned to the node.
	PodCIDR string `json:"podCIDR,omitempty"`
	// podCIDRs represents the IP ranges assigned to the node for usage by Pods on that node. If this
	// field is specified, the 0th entry must match the podCIDR field. It may contain at most 1 value for
	// each of IPv4 and IPv6.
	PodCIDRs []string `json:"podCIDRs"`
	// ID of the node assigned by the cloud provider in the format: <ProviderName>://<ProviderSpecificNodeID>
	ProviderID string `json:"providerID,omitempty"`
	// Unschedulable controls node schedulability of new pods. By default, node is schedulable.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#manual-node-administration
	Unschedulable bool `json:"unschedulable,omitempty"`
	// If specified, the node's taints.
	Taints []Taint `json:"taints"`
	// Deprecated: Previously used to specify the source of the node's configuration for the DynamicKubeletConfig feature. This feature is removed.
	ConfigSource *NodeConfigSource `json:"configSource,omitempty"`
	// Deprecated. Not all kubelets will set this field. Remove field after 1.13.
	// see: https://issues.k8s.io/61966
	DoNotUseExternalID string `json:"externalID,omitempty"`
}

func (in *NodeSpec) DeepCopyInto(out *NodeSpec) {
	*out = *in
	if in.PodCIDRs != nil {
		t := make([]string, len(in.PodCIDRs))
		copy(t, in.PodCIDRs)
		out.PodCIDRs = t
	}
	if in.Taints != nil {
		l := make([]Taint, len(in.Taints))
		for i := range in.Taints {
			in.Taints[i].DeepCopyInto(&l[i])
		}
		out.Taints = l
	}
	if in.ConfigSource != nil {
		in, out := &in.ConfigSource, &out.ConfigSource
		*out = new(NodeConfigSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NodeSpec) DeepCopy() *NodeSpec {
	if in == nil {
		return nil
	}
	out := new(NodeSpec)
	in.DeepCopyInto(out)
	return out
}

type NodeStatus struct {
	// Capacity represents the total resources of a node.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#capacity
	Capacity map[string]apiresource.Quantity `json:"capacity,omitempty"`
	// Allocatable represents the resources of a node that are available for scheduling.
	// Defaults to Capacity.
	Allocatable map[string]apiresource.Quantity `json:"allocatable,omitempty"`
	// NodePhase is the recently observed lifecycle phase of the node.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#phase
	// The field is never populated, and now is deprecated.
	Phase NodePhase `json:"phase,omitempty"`
	// Conditions is an array of current observed node conditions.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#condition
	Conditions []NodeCondition `json:"conditions"`
	// List of addresses reachable to the node.
	// Queried from cloud provider, if available.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#addresses
	// Note: This field is declared as mergeable, but the merge key is not sufficiently
	// unique, which can cause data corruption when it is merged. Callers should instead
	// use a full-replacement patch. See https://pr.k8s.io/79391 for an example.
	// Consumers should assume that addresses can change during the
	// lifetime of a Node. However, there are some exceptions where this may not
	// be possible, such as Pods that inherit a Node's address in its own status or
	// consumers of the downward API (status.hostIP).
	Addresses []NodeAddress `json:"addresses"`
	// Endpoints of daemons running on the Node.
	DaemonEndpoints *NodeDaemonEndpoints `json:"daemonEndpoints,omitempty"`
	// Set of ids/uuids to uniquely identify the node.
	// More info: https://kubernetes.io/docs/concepts/nodes/node/#info
	NodeInfo *NodeSystemInfo `json:"nodeInfo,omitempty"`
	// List of container images on this node
	Images []ContainerImage `json:"images"`
	// List of attachable volumes in use (mounted) by the node.
	VolumesInUse []string `json:"volumesInUse"`
	// List of volumes that are attached to the node.
	VolumesAttached []AttachedVolume `json:"volumesAttached"`
	// Status of the config assigned to the node via the dynamic Kubelet config feature.
	Config *NodeConfigStatus `json:"config,omitempty"`
}

func (in *NodeStatus) DeepCopyInto(out *NodeStatus) {
	*out = *in
	if in.Capacity != nil {
		in, out := &in.Capacity, &out.Capacity
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Allocatable != nil {
		in, out := &in.Allocatable, &out.Allocatable
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Conditions != nil {
		l := make([]NodeCondition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
	if in.Addresses != nil {
		l := make([]NodeAddress, len(in.Addresses))
		for i := range in.Addresses {
			in.Addresses[i].DeepCopyInto(&l[i])
		}
		out.Addresses = l
	}
	if in.DaemonEndpoints != nil {
		in, out := &in.DaemonEndpoints, &out.DaemonEndpoints
		*out = new(NodeDaemonEndpoints)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeInfo != nil {
		in, out := &in.NodeInfo, &out.NodeInfo
		*out = new(NodeSystemInfo)
		(*in).DeepCopyInto(*out)
	}
	if in.Images != nil {
		l := make([]ContainerImage, len(in.Images))
		for i := range in.Images {
			in.Images[i].DeepCopyInto(&l[i])
		}
		out.Images = l
	}
	if in.VolumesInUse != nil {
		t := make([]string, len(in.VolumesInUse))
		copy(t, in.VolumesInUse)
		out.VolumesInUse = t
	}
	if in.VolumesAttached != nil {
		l := make([]AttachedVolume, len(in.VolumesAttached))
		for i := range in.VolumesAttached {
			in.VolumesAttached[i].DeepCopyInto(&l[i])
		}
		out.VolumesAttached = l
	}
	if in.Config != nil {
		in, out := &in.Config, &out.Config
		*out = new(NodeConfigStatus)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NodeStatus) DeepCopy() *NodeStatus {
	if in == nil {
		return nil
	}
	out := new(NodeStatus)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeSpec struct {
	// capacity is the description of the persistent volume's resources and capacity.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#capacity
	Capacity map[string]apiresource.Quantity `json:"capacity,omitempty"`
	// persistentVolumeSource is the actual volume backing the persistent volume.
	PersistentVolumeSource `json:",inline"`
	// accessModes contains all ways the volume can be mounted.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes
	AccessModes []PersistentVolumeAccessMode `json:"accessModes"`
	// claimRef is part of a bi-directional binding between PersistentVolume and PersistentVolumeClaim.
	// Expected to be non-nil when bound.
	// claim.VolumeName is the authoritative bind between PV and PVC.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#binding
	ClaimRef *ObjectReference `json:"claimRef,omitempty"`
	// persistentVolumeReclaimPolicy defines what happens to a persistent volume when released from its claim.
	// Valid options are Retain (default for manually created PersistentVolumes), Delete (default
	// for dynamically provisioned PersistentVolumes), and Recycle (deprecated).
	// Recycle must be supported by the volume plugin underlying this PersistentVolume.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#reclaiming
	PersistentVolumeReclaimPolicy PersistentVolumeReclaimPolicy `json:"persistentVolumeReclaimPolicy,omitempty"`
	// storageClassName is the name of StorageClass to which this persistent volume belongs. Empty value
	// means that this volume does not belong to any StorageClass.
	StorageClassName string `json:"storageClassName,omitempty"`
	// mountOptions is the list of mount options, e.g. ["ro", "soft"]. Not validated - mount will
	// simply fail if one is invalid.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes/#mount-options
	MountOptions []string `json:"mountOptions"`
	// volumeMode defines if a volume is intended to be used with a formatted filesystem
	// or to remain in raw block state. Value of Filesystem is implied when not included in spec.
	VolumeMode PersistentVolumeMode `json:"volumeMode,omitempty"`
	// nodeAffinity defines constraints that limit what nodes this volume can be accessed from.
	// This field influences the scheduling of pods that use this volume.
	NodeAffinity *VolumeNodeAffinity `json:"nodeAffinity,omitempty"`
}

func (in *PersistentVolumeSpec) DeepCopyInto(out *PersistentVolumeSpec) {
	*out = *in
	if in.Capacity != nil {
		in, out := &in.Capacity, &out.Capacity
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	out.PersistentVolumeSource = in.PersistentVolumeSource
	if in.AccessModes != nil {
		t := make([]PersistentVolumeAccessMode, len(in.AccessModes))
		copy(t, in.AccessModes)
		out.AccessModes = t
	}
	if in.ClaimRef != nil {
		in, out := &in.ClaimRef, &out.ClaimRef
		*out = new(ObjectReference)
		(*in).DeepCopyInto(*out)
	}
	if in.MountOptions != nil {
		t := make([]string, len(in.MountOptions))
		copy(t, in.MountOptions)
		out.MountOptions = t
	}
	if in.NodeAffinity != nil {
		in, out := &in.NodeAffinity, &out.NodeAffinity
		*out = new(VolumeNodeAffinity)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentVolumeSpec) DeepCopy() *PersistentVolumeSpec {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeSpec)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeStatus struct {
	// phase indicates if a volume is available, bound to a claim, or released by a claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#phase
	Phase PersistentVolumePhase `json:"phase,omitempty"`
	// message is a human-readable message indicating details about why the volume is in this state.
	Message string `json:"message,omitempty"`
	// reason is a brief CamelCase string that describes any failure and is meant
	// for machine parsing and tidy display in the CLI.
	Reason string `json:"reason,omitempty"`
}

func (in *PersistentVolumeStatus) DeepCopyInto(out *PersistentVolumeStatus) {
	*out = *in
}

func (in *PersistentVolumeStatus) DeepCopy() *PersistentVolumeStatus {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeStatus)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeClaimSpec struct {
	// accessModes contains the desired access modes the volume should have.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	AccessModes []PersistentVolumeAccessMode `json:"accessModes"`
	// selector is a label query over volumes to consider for binding.
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
	// resources represents the minimum resources the volume should have.
	// If RecoverVolumeExpansionFailure feature is enabled users are allowed to specify resource requirements
	// that are lower than previous value but must still be higher than capacity recorded in the
	// status field of the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#resources
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// volumeName is the binding reference to the PersistentVolume backing this claim.
	VolumeName string `json:"volumeName,omitempty"`
	// storageClassName is the name of the StorageClass required by the claim.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#class-1
	StorageClassName string `json:"storageClassName,omitempty"`
	// volumeMode defines what type of volume is required by the claim.
	// Value of Filesystem is implied when not included in claim spec.
	VolumeMode PersistentVolumeMode `json:"volumeMode,omitempty"`
	// dataSource field can be used to specify either:
	// * An existing VolumeSnapshot object (snapshot.storage.k8s.io/VolumeSnapshot)
	// * An existing PVC (PersistentVolumeClaim)
	// If the provisioner or an external controller can support the specified data source,
	// it will create a new volume based on the contents of the specified data source.
	// When the AnyVolumeDataSource feature gate is enabled, dataSource contents will be copied to dataSourceRef,
	// and dataSourceRef contents will be copied to dataSource when dataSourceRef.namespace is not specified.
	// If the namespace is specified, then dataSourceRef will not be copied to dataSource.
	DataSource *TypedLocalObjectReference `json:"dataSource,omitempty"`
	// dataSourceRef specifies the object from which to populate the volume with data, if a non-empty
	// volume is desired. This may be any object from a non-empty API group (non
	// core object) or a PersistentVolumeClaim object.
	// When this field is specified, volume binding will only succeed if the type of
	// the specified object matches some installed volume populator or dynamic
	// provisioner.
	// This field will replace the functionality of the dataSource field and as such
	// if both fields are non-empty, they must have the same value. For backwards
	// compatibility, when namespace isn't specified in dataSourceRef,
	// both fields (dataSource and dataSourceRef) will be set to the same
	// value automatically if one of them is empty and the other is non-empty.
	// When namespace is specified in dataSourceRef,
	// dataSource isn't set to the same value and must be empty.
	// There are three important differences between dataSource and dataSourceRef:
	// * While dataSource only allows two specific types of objects, dataSourceRef
	// allows any non-core object, as well as PersistentVolumeClaim objects.
	// * While dataSource ignores disallowed values (dropping them), dataSourceRef
	// preserves all values, and generates an error if a disallowed value is
	// specified.
	// * While dataSource only allows local objects, dataSourceRef allows objects
	// in any namespaces.
	// (Beta) Using this field requires the AnyVolumeDataSource feature gate to be enabled.
	// (Alpha) Using the namespace field of dataSourceRef requires the CrossNamespaceVolumeDataSource feature gate to be enabled.
	DataSourceRef *TypedObjectReference `json:"dataSourceRef,omitempty"`
}

func (in *PersistentVolumeClaimSpec) DeepCopyInto(out *PersistentVolumeClaimSpec) {
	*out = *in
	if in.AccessModes != nil {
		t := make([]PersistentVolumeAccessMode, len(in.AccessModes))
		copy(t, in.AccessModes)
		out.AccessModes = t
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.DataSource != nil {
		in, out := &in.DataSource, &out.DataSource
		*out = new(TypedLocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
	if in.DataSourceRef != nil {
		in, out := &in.DataSourceRef, &out.DataSourceRef
		*out = new(TypedObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentVolumeClaimSpec) DeepCopy() *PersistentVolumeClaimSpec {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimSpec)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeClaimStatus struct {
	// phase represents the current phase of PersistentVolumeClaim.
	Phase PersistentVolumeClaimPhase `json:"phase,omitempty"`
	// accessModes contains the actual access modes the volume backing the PVC has.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#access-modes-1
	AccessModes []PersistentVolumeAccessMode `json:"accessModes"`
	// capacity represents the actual resources of the underlying volume.
	Capacity map[string]apiresource.Quantity `json:"capacity,omitempty"`
	// conditions is the current Condition of persistent volume claim. If underlying persistent volume is being
	// resized then the Condition will be set to 'ResizeStarted'.
	Conditions []PersistentVolumeClaimCondition `json:"conditions"`
	// allocatedResources is the storage resource within AllocatedResources tracks the capacity allocated to a PVC. It may
	// be larger than the actual capacity when a volume expansion operation is requested.
	// For storage quota, the larger value from allocatedResources and PVC.spec.resources is used.
	// If allocatedResources is not set, PVC.spec.resources alone is used for quota calculation.
	// If a volume expansion capacity request is lowered, allocatedResources is only
	// lowered if there are no expansion operations in progress and if the actual volume capacity
	// is equal or lower than the requested capacity.
	// This is an alpha field and requires enabling RecoverVolumeExpansionFailure feature.
	AllocatedResources map[string]apiresource.Quantity `json:"allocatedResources,omitempty"`
	// resizeStatus stores status of resize operation.
	// ResizeStatus is not set by default but when expansion is complete resizeStatus is set to empty
	// string by resize controller or kubelet.
	// This is an alpha field and requires enabling RecoverVolumeExpansionFailure feature.
	ResizeStatus PersistentVolumeClaimResizeStatus `json:"resizeStatus,omitempty"`
}

func (in *PersistentVolumeClaimStatus) DeepCopyInto(out *PersistentVolumeClaimStatus) {
	*out = *in
	if in.AccessModes != nil {
		t := make([]PersistentVolumeAccessMode, len(in.AccessModes))
		copy(t, in.AccessModes)
		out.AccessModes = t
	}
	if in.Capacity != nil {
		in, out := &in.Capacity, &out.Capacity
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Conditions != nil {
		l := make([]PersistentVolumeClaimCondition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
	if in.AllocatedResources != nil {
		in, out := &in.AllocatedResources, &out.AllocatedResources
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *PersistentVolumeClaimStatus) DeepCopy() *PersistentVolumeClaimStatus {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimStatus)
	in.DeepCopyInto(out)
	return out
}

type PodSpec struct {
	// List of volumes that can be mounted by containers belonging to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes
	Volumes []Volume `json:"volumes"`
	// List of initialization containers belonging to the pod.
	// Init containers are executed in order prior to containers being started. If any
	// init container fails, the pod is considered to have failed and is handled according
	// to its restartPolicy. The name for an init container or normal container must be
	// unique among all containers.
	// Init containers may not have Lifecycle actions, Readiness probes, Liveness probes, or Startup probes.
	// The resourceRequirements of an init container are taken into account during scheduling
	// by finding the highest request/limit for each resource type, and then using the max of
	// of that value or the sum of the normal containers. Limits are applied to init containers
	// in a similar fashion.
	// Init containers cannot currently be added or removed.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/init-containers/
	InitContainers []Container `json:"initContainers"`
	// List of containers belonging to the pod.
	// Containers cannot currently be added or removed.
	// There must be at least one container in a Pod.
	// Cannot be updated.
	Containers []Container `json:"containers"`
	// List of ephemeral containers run in this pod. Ephemeral containers may be run in an existing
	// pod to perform user-initiated actions such as debugging. This list cannot be specified when
	// creating a pod, and it cannot be modified by updating the pod spec. In order to add an
	// ephemeral container to an existing pod, use the pod's ephemeralcontainers subresource.
	EphemeralContainers []EphemeralContainer `json:"ephemeralContainers"`
	// Restart policy for all containers within the pod.
	// One of Always, OnFailure, Never. In some contexts, only a subset of those values may be permitted.
	// Default to Always.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#restart-policy
	RestartPolicy RestartPolicy `json:"restartPolicy,omitempty"`
	// Optional duration in seconds the pod needs to terminate gracefully. May be decreased in delete request.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// If this value is nil, the default grace period will be used instead.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// Defaults to 30 seconds.
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`
	// Optional duration in seconds the pod may be active on the node relative to
	// StartTime before the system will actively try to mark it failed and kill associated containers.
	// Value must be a positive integer.
	ActiveDeadlineSeconds int64 `json:"activeDeadlineSeconds,omitempty"`
	// Set DNS policy for the pod.
	// Defaults to "ClusterFirst".
	// Valid values are 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'.
	// DNS parameters given in DNSConfig will be merged with the policy selected with DNSPolicy.
	// To have DNS options set along with hostNetwork, you have to specify DNS policy
	// explicitly to 'ClusterFirstWithHostNet'.
	DNSPolicy DNSPolicy `json:"dnsPolicy,omitempty"`
	// NodeSelector is a selector which must be true for the pod to fit on a node.
	// Selector which must match a node's labels for the pod to be scheduled on that node.
	// More info: https://kubernetes.io/docs/concepts/configuration/assign-pod-node/
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use to run this pod.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/configure-service-account/
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// DeprecatedServiceAccount is a depreciated alias for ServiceAccountName.
	// Deprecated: Use serviceAccountName instead.
	DeprecatedServiceAccount string `json:"serviceAccount,omitempty"`
	// AutomountServiceAccountToken indicates whether a service account token should be automatically mounted.
	AutomountServiceAccountToken bool `json:"automountServiceAccountToken,omitempty"`
	// NodeName is a request to schedule this pod onto a specific node. If it is non-empty,
	// the scheduler simply schedules this pod onto that node, assuming that it fits resource
	// requirements.
	NodeName string `json:"nodeName,omitempty"`
	// Host networking requested for this pod. Use the host's network namespace.
	// If this option is set, the ports that will be used must be specified.
	// Default to false.
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// Use the host's pid namespace.
	// Optional: Default to false.
	HostPID bool `json:"hostPID,omitempty"`
	// Use the host's ipc namespace.
	// Optional: Default to false.
	HostIPC bool `json:"hostIPC,omitempty"`
	// Share a single process namespace between all of the containers in a pod.
	// When this is set containers will be able to view and signal processes from other containers
	// in the same pod, and the first process in each container will not be assigned PID 1.
	// HostPID and ShareProcessNamespace cannot both be set.
	// Optional: Default to false.
	ShareProcessNamespace bool `json:"shareProcessNamespace,omitempty"`
	// SecurityContext holds pod-level security attributes and common container settings.
	// Optional: Defaults to empty.  See type description for default values of each field.
	SecurityContext *PodSecurityContext `json:"securityContext,omitempty"`
	// ImagePullSecrets is an optional list of references to secrets in the same namespace to use for pulling any of the images used by this PodSpec.
	// If specified, these secrets will be passed to individual puller implementations for them to use.
	// More info: https://kubernetes.io/docs/concepts/containers/images#specifying-imagepullsecrets-on-a-pod
	ImagePullSecrets []LocalObjectReference `json:"imagePullSecrets"`
	// Specifies the hostname of the Pod
	// If not specified, the pod's hostname will be set to a system-defined value.
	Hostname string `json:"hostname,omitempty"`
	// If specified, the fully qualified Pod hostname will be "<hostname>.<subdomain>.<pod namespace>.svc.<cluster domain>".
	// If not specified, the pod will not have a domainname at all.
	Subdomain string `json:"subdomain,omitempty"`
	// If specified, the pod's scheduling constraints
	Affinity *Affinity `json:"affinity,omitempty"`
	// If specified, the pod will be dispatched by specified scheduler.
	// If not specified, the pod will be dispatched by default scheduler.
	SchedulerName string `json:"schedulerName,omitempty"`
	// If specified, the pod's tolerations.
	Tolerations []Toleration `json:"tolerations"`
	// HostAliases is an optional list of hosts and IPs that will be injected into the pod's hosts
	// file if specified. This is only valid for non-hostNetwork pods.
	HostAliases []HostAlias `json:"hostAliases"`
	// If specified, indicates the pod's priority. "system-node-critical" and
	// "system-cluster-critical" are two special keywords which indicate the
	// highest priorities with the former being the highest priority. Any other
	// name must be defined by creating a PriorityClass object with that name.
	// If not specified, the pod priority will be default or zero if there is no
	// default.
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// The priority value. Various system components use this field to find the
	// priority of the pod. When Priority Admission Controller is enabled, it
	// prevents users from setting this field. The admission controller populates
	// this field from PriorityClassName.
	// The higher the value, the higher the priority.
	Priority int `json:"priority,omitempty"`
	// Specifies the DNS parameters of a pod.
	// Parameters specified here will be merged to the generated DNS
	// configuration based on DNSPolicy.
	DNSConfig *PodDNSConfig `json:"dnsConfig,omitempty"`
	// If specified, all readiness gates will be evaluated for pod readiness.
	// A pod is ready when all its containers are ready AND
	// all conditions specified in the readiness gates have status equal to "True"
	// More info: https://git.k8s.io/enhancements/keps/sig-network/580-pod-readiness-gates
	ReadinessGates []PodReadinessGate `json:"readinessGates"`
	// RuntimeClassName refers to a RuntimeClass object in the node.k8s.io group, which should be used
	// to run this pod.  If no RuntimeClass resource matches the named class, the pod will not be run.
	// If unset or empty, the "legacy" RuntimeClass will be used, which is an implicit class with an
	// empty definition that uses the default runtime handler.
	// More info: https://git.k8s.io/enhancements/keps/sig-node/585-runtime-class
	RuntimeClassName string `json:"runtimeClassName,omitempty"`
	// EnableServiceLinks indicates whether information about services should be injected into pod's
	// environment variables, matching the syntax of Docker links.
	// Optional: Defaults to true.
	EnableServiceLinks bool `json:"enableServiceLinks,omitempty"`
	// PreemptionPolicy is the Policy for preempting pods with lower priority.
	// One of Never, PreemptLowerPriority.
	// Defaults to PreemptLowerPriority if unset.
	PreemptionPolicy PreemptionPolicy `json:"preemptionPolicy,omitempty"`
	// Overhead represents the resource overhead associated with running a pod for a given RuntimeClass.
	// This field will be autopopulated at admission time by the RuntimeClass admission controller. If
	// the RuntimeClass admission controller is enabled, overhead must not be set in Pod create requests.
	// The RuntimeClass admission controller will reject Pod create requests which have the overhead already
	// set. If RuntimeClass is configured and selected in the PodSpec, Overhead will be set to the value
	// defined in the corresponding RuntimeClass, otherwise it will remain unset and treated as zero.
	// More info: https://git.k8s.io/enhancements/keps/sig-node/688-pod-overhead/README.md
	Overhead map[string]apiresource.Quantity `json:"overhead,omitempty"`
	// TopologySpreadConstraints describes how a group of pods ought to spread across topology
	// domains. Scheduler will schedule pods in a way which abides by the constraints.
	// All topologySpreadConstraints are ANDed.
	TopologySpreadConstraints []TopologySpreadConstraint `json:"topologySpreadConstraints"`
	// If true the pod's hostname will be configured as the pod's FQDN, rather than the leaf name (the default).
	// In Linux containers, this means setting the FQDN in the hostname field of the kernel (the nodename field of struct utsname).
	// In Windows containers, this means setting the registry value of hostname for the registry key HKEY_LOCAL_MACHINE\\SYSTEM\\CurrentControlSet\\Services\\Tcpip\\Parameters to FQDN.
	// If a pod does not have FQDN, this has no effect.
	// Default to false.
	SetHostnameAsFQDN bool `json:"setHostnameAsFQDN,omitempty"`
	// Specifies the OS of the containers in the pod.
	// Some pod and container fields are restricted if this is set.
	// If the OS field is set to linux, the following fields must be unset:
	// -securityContext.windowsOptions
	// If the OS field is set to windows, following fields must be unset:
	// - spec.hostPID
	// - spec.hostIPC
	// - spec.hostUsers
	// - spec.securityContext.seLinuxOptions
	// - spec.securityContext.seccompProfile
	// - spec.securityContext.fsGroup
	// - spec.securityContext.fsGroupChangePolicy
	// - spec.securityContext.sysctls
	// - spec.shareProcessNamespace
	// - spec.securityContext.runAsUser
	// - spec.securityContext.runAsGroup
	// - spec.securityContext.supplementalGroups
	// - spec.containers[*].securityContext.seLinuxOptions
	// - spec.containers[*].securityContext.seccompProfile
	// - spec.containers[*].securityContext.capabilities
	// - spec.containers[*].securityContext.readOnlyRootFilesystem
	// - spec.containers[*].securityContext.privileged
	// - spec.containers[*].securityContext.allowPrivilegeEscalation
	// - spec.containers[*].securityContext.procMount
	// - spec.containers[*].securityContext.runAsUser
	// - spec.containers[*].securityContext.runAsGroup
	OS *PodOS `json:"os,omitempty"`
	// Use the host's user namespace.
	// Optional: Default to true.
	// If set to true or not present, the pod will be run in the host user namespace, useful
	// for when the pod needs a feature only available to the host user namespace, such as
	// loading a kernel module with CAP_SYS_MODULE.
	// When set to false, a new userns is created for the pod. Setting false is useful for
	// mitigating container breakout vulnerabilities even allowing users to run their
	// containers as root without actually having root privileges on the host.
	// This field is alpha-level and is only honored by servers that enable the UserNamespacesSupport feature.
	HostUsers bool `json:"hostUsers,omitempty"`
	// SchedulingGates is an opaque list of values that if specified will block scheduling the pod.
	// If schedulingGates is not empty, the pod will stay in the SchedulingGated state and the
	// scheduler will not attempt to schedule the pod.
	// SchedulingGates can only be set at pod creation time, and be removed only afterwards.
	// This is a beta feature enabled by the PodSchedulingReadiness feature gate.
	SchedulingGates []PodSchedulingGate `json:"schedulingGates"`
	// ResourceClaims defines which ResourceClaims must be allocated
	// and reserved before the Pod is allowed to start. The resources
	// will be made available to those containers which consume them
	// by name.
	// This is an alpha field and requires enabling the
	// DynamicResourceAllocation feature gate.
	// This field is immutable.
	ResourceClaims []PodResourceClaim `json:"resourceClaims"`
}

func (in *PodSpec) DeepCopyInto(out *PodSpec) {
	*out = *in
	if in.Volumes != nil {
		l := make([]Volume, len(in.Volumes))
		for i := range in.Volumes {
			in.Volumes[i].DeepCopyInto(&l[i])
		}
		out.Volumes = l
	}
	if in.InitContainers != nil {
		l := make([]Container, len(in.InitContainers))
		for i := range in.InitContainers {
			in.InitContainers[i].DeepCopyInto(&l[i])
		}
		out.InitContainers = l
	}
	if in.Containers != nil {
		l := make([]Container, len(in.Containers))
		for i := range in.Containers {
			in.Containers[i].DeepCopyInto(&l[i])
		}
		out.Containers = l
	}
	if in.EphemeralContainers != nil {
		l := make([]EphemeralContainer, len(in.EphemeralContainers))
		for i := range in.EphemeralContainers {
			in.EphemeralContainers[i].DeepCopyInto(&l[i])
		}
		out.EphemeralContainers = l
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(PodSecurityContext)
		(*in).DeepCopyInto(*out)
	}
	if in.ImagePullSecrets != nil {
		l := make([]LocalObjectReference, len(in.ImagePullSecrets))
		for i := range in.ImagePullSecrets {
			in.ImagePullSecrets[i].DeepCopyInto(&l[i])
		}
		out.ImagePullSecrets = l
	}
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.Tolerations != nil {
		l := make([]Toleration, len(in.Tolerations))
		for i := range in.Tolerations {
			in.Tolerations[i].DeepCopyInto(&l[i])
		}
		out.Tolerations = l
	}
	if in.HostAliases != nil {
		l := make([]HostAlias, len(in.HostAliases))
		for i := range in.HostAliases {
			in.HostAliases[i].DeepCopyInto(&l[i])
		}
		out.HostAliases = l
	}
	if in.DNSConfig != nil {
		in, out := &in.DNSConfig, &out.DNSConfig
		*out = new(PodDNSConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ReadinessGates != nil {
		l := make([]PodReadinessGate, len(in.ReadinessGates))
		for i := range in.ReadinessGates {
			in.ReadinessGates[i].DeepCopyInto(&l[i])
		}
		out.ReadinessGates = l
	}
	if in.Overhead != nil {
		in, out := &in.Overhead, &out.Overhead
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.TopologySpreadConstraints != nil {
		l := make([]TopologySpreadConstraint, len(in.TopologySpreadConstraints))
		for i := range in.TopologySpreadConstraints {
			in.TopologySpreadConstraints[i].DeepCopyInto(&l[i])
		}
		out.TopologySpreadConstraints = l
	}
	if in.OS != nil {
		in, out := &in.OS, &out.OS
		*out = new(PodOS)
		(*in).DeepCopyInto(*out)
	}
	if in.SchedulingGates != nil {
		l := make([]PodSchedulingGate, len(in.SchedulingGates))
		for i := range in.SchedulingGates {
			in.SchedulingGates[i].DeepCopyInto(&l[i])
		}
		out.SchedulingGates = l
	}
	if in.ResourceClaims != nil {
		l := make([]PodResourceClaim, len(in.ResourceClaims))
		for i := range in.ResourceClaims {
			in.ResourceClaims[i].DeepCopyInto(&l[i])
		}
		out.ResourceClaims = l
	}
}

func (in *PodSpec) DeepCopy() *PodSpec {
	if in == nil {
		return nil
	}
	out := new(PodSpec)
	in.DeepCopyInto(out)
	return out
}

type PodStatus struct {
	// The phase of a Pod is a simple, high-level summary of where the Pod is in its lifecycle.
	// The conditions array, the reason and message fields, and the individual container status
	// arrays contain more detail about the pod's status.
	// There are five possible phase values:
	// Pending: The pod has been accepted by the Kubernetes system, but one or more of the
	// container images has not been created. This includes time before being scheduled as
	// well as time spent downloading images over the network, which could take a while.
	// Running: The pod has been bound to a node, and all of the containers have been created.
	// At least one container is still running, or is in the process of starting or restarting.
	// Succeeded: All containers in the pod have terminated in success, and will not be restarted.
	// Failed: All containers in the pod have terminated, and at least one container has
	// terminated in failure. The container either exited with non-zero status or was terminated
	// by the system.
	// Unknown: For some reason the state of the pod could not be obtained, typically due to an
	// error in communicating with the host of the pod.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-phase
	Phase PodPhase `json:"phase,omitempty"`
	// Current service state of pod.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Conditions []PodCondition `json:"conditions"`
	// A human readable message indicating details about why the pod is in this condition.
	Message string `json:"message,omitempty"`
	// A brief CamelCase message indicating details about why the pod is in this state.
	// e.g. 'Evicted'
	Reason string `json:"reason,omitempty"`
	// nominatedNodeName is set only when this pod preempts other pods on the node, but it cannot be
	// scheduled right away as preemption victims receive their graceful termination periods.
	// This field does not guarantee that the pod will be scheduled on this node. Scheduler may decide
	// to place the pod elsewhere if other nodes become available sooner. Scheduler may also decide to
	// give the resources on this node to a higher priority pod that is created after preemption.
	// As a result, this field may be different than PodSpec.nodeName when the pod is
	// scheduled.
	NominatedNodeName string `json:"nominatedNodeName,omitempty"`
	// IP address of the host to which the pod is assigned. Empty if not yet scheduled.
	HostIP string `json:"hostIP,omitempty"`
	// IP address allocated to the pod. Routable at least within the cluster.
	// Empty if not yet allocated.
	PodIP string `json:"podIP,omitempty"`
	// podIPs holds the IP addresses allocated to the pod. If this field is specified, the 0th entry must
	// match the podIP field. Pods may be allocated at most 1 value for each of IPv4 and IPv6. This list
	// is empty if no IPs have been allocated yet.
	PodIPs []PodIP `json:"podIPs"`
	// RFC 3339 date and time at which the object was acknowledged by the Kubelet.
	// This is before the Kubelet pulled the container image(s) for the pod.
	StartTime *metav1.Time `json:"startTime,omitempty"`
	// The list has one entry per init container in the manifest. The most recent successful
	// init container will have ready = true, the most recently started container will have
	// startTime set.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-and-container-status
	InitContainerStatuses []ContainerStatus `json:"initContainerStatuses"`
	// The list has one entry per container in the manifest.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-and-container-status
	ContainerStatuses []ContainerStatus `json:"containerStatuses"`
	// The Quality of Service (QOS) classification assigned to the pod based on resource requirements
	// See PodQOSClass type for available QOS classes
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-qos/#quality-of-service-classes
	QOSClass PodQOSClass `json:"qosClass,omitempty"`
	// Status for any ephemeral containers that have run in this pod.
	EphemeralContainerStatuses []ContainerStatus `json:"ephemeralContainerStatuses"`
	// Status of resources resize desired for pod's containers.
	// It is empty if no resources resize is pending.
	// Any changes to container resources will automatically set this to "Proposed"
	Resize PodResizeStatus `json:"resize,omitempty"`
}

func (in *PodStatus) DeepCopyInto(out *PodStatus) {
	*out = *in
	if in.Conditions != nil {
		l := make([]PodCondition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
	if in.PodIPs != nil {
		l := make([]PodIP, len(in.PodIPs))
		for i := range in.PodIPs {
			in.PodIPs[i].DeepCopyInto(&l[i])
		}
		out.PodIPs = l
	}
	if in.StartTime != nil {
		in, out := &in.StartTime, &out.StartTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.InitContainerStatuses != nil {
		l := make([]ContainerStatus, len(in.InitContainerStatuses))
		for i := range in.InitContainerStatuses {
			in.InitContainerStatuses[i].DeepCopyInto(&l[i])
		}
		out.InitContainerStatuses = l
	}
	if in.ContainerStatuses != nil {
		l := make([]ContainerStatus, len(in.ContainerStatuses))
		for i := range in.ContainerStatuses {
			in.ContainerStatuses[i].DeepCopyInto(&l[i])
		}
		out.ContainerStatuses = l
	}
	if in.EphemeralContainerStatuses != nil {
		l := make([]ContainerStatus, len(in.EphemeralContainerStatuses))
		for i := range in.EphemeralContainerStatuses {
			in.EphemeralContainerStatuses[i].DeepCopyInto(&l[i])
		}
		out.EphemeralContainerStatuses = l
	}
}

func (in *PodStatus) DeepCopy() *PodStatus {
	if in == nil {
		return nil
	}
	out := new(PodStatus)
	in.DeepCopyInto(out)
	return out
}

type PodTemplateSpec struct {
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#metadata
	ObjectMeta *metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the pod.
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
	Spec *PodSpec `json:"spec,omitempty"`
}

func (in *PodTemplateSpec) DeepCopyInto(out *PodTemplateSpec) {
	*out = *in
	if in.ObjectMeta != nil {
		in, out := &in.ObjectMeta, &out.ObjectMeta
		*out = new(metav1.ObjectMeta)
		(*in).DeepCopyInto(*out)
	}
	if in.Spec != nil {
		in, out := &in.Spec, &out.Spec
		*out = new(PodSpec)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodTemplateSpec) DeepCopy() *PodTemplateSpec {
	if in == nil {
		return nil
	}
	out := new(PodTemplateSpec)
	in.DeepCopyInto(out)
	return out
}

type ReplicationControllerSpec struct {
	// Replicas is the number of desired replicas.
	// This is a pointer to distinguish between explicit zero and unspecified.
	// Defaults to 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#what-is-a-replicationcontroller
	Replicas int `json:"replicas,omitempty"`
	// Minimum number of seconds for which a newly created pod should be ready
	// without any of its container crashing, for it to be considered available.
	// Defaults to 0 (pod will be considered available as soon as it is ready)
	MinReadySeconds int `json:"minReadySeconds,omitempty"`
	// Selector is a label query over pods that should match the Replicas count.
	// If Selector is empty, it is defaulted to the labels present on the Pod template.
	// Label keys and values that must match in order to be controlled by this replication
	// controller, if empty defaulted to labels on Pod template.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors
	Selector map[string]string `json:"selector,omitempty"`
	// Template is the object that describes the pod that will be created if
	// insufficient replicas are detected. This takes precedence over a TemplateRef.
	// The only allowed template.spec.restartPolicy value is "Always".
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#pod-template
	Template *PodTemplateSpec `json:"template,omitempty"`
}

func (in *ReplicationControllerSpec) DeepCopyInto(out *ReplicationControllerSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Template != nil {
		in, out := &in.Template, &out.Template
		*out = new(PodTemplateSpec)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ReplicationControllerSpec) DeepCopy() *ReplicationControllerSpec {
	if in == nil {
		return nil
	}
	out := new(ReplicationControllerSpec)
	in.DeepCopyInto(out)
	return out
}

type ReplicationControllerStatus struct {
	// Replicas is the most recently observed number of replicas.
	// More info: https://kubernetes.io/docs/concepts/workloads/controllers/replicationcontroller#what-is-a-replicationcontroller
	Replicas int `json:"replicas"`
	// The number of pods that have labels matching the labels of the pod template of the replication controller.
	FullyLabeledReplicas int `json:"fullyLabeledReplicas,omitempty"`
	// The number of ready replicas for this replication controller.
	ReadyReplicas int `json:"readyReplicas,omitempty"`
	// The number of available replicas (ready for at least minReadySeconds) for this replication controller.
	AvailableReplicas int `json:"availableReplicas,omitempty"`
	// ObservedGeneration reflects the generation of the most recently observed replication controller.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// Represents the latest available observations of a replication controller's current state.
	Conditions []ReplicationControllerCondition `json:"conditions"`
}

func (in *ReplicationControllerStatus) DeepCopyInto(out *ReplicationControllerStatus) {
	*out = *in
	if in.Conditions != nil {
		l := make([]ReplicationControllerCondition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
}

func (in *ReplicationControllerStatus) DeepCopy() *ReplicationControllerStatus {
	if in == nil {
		return nil
	}
	out := new(ReplicationControllerStatus)
	in.DeepCopyInto(out)
	return out
}

type ResourceQuotaSpec struct {
	// hard is the set of desired hard limits for each named resource.
	// More info: https://kubernetes.io/docs/concepts/policy/resource-quotas/
	Hard map[string]apiresource.Quantity `json:"hard,omitempty"`
	// A collection of filters that must match each object tracked by a quota.
	// If not specified, the quota matches all objects.
	Scopes []ResourceQuotaScope `json:"scopes"`
	// scopeSelector is also a collection of filters like scopes that must match each object tracked by a quota
	// but expressed using ScopeSelectorOperator in combination with possible values.
	// For a resource to match, both scopes AND scopeSelector (if specified in spec), must be matched.
	ScopeSelector *ScopeSelector `json:"scopeSelector,omitempty"`
}

func (in *ResourceQuotaSpec) DeepCopyInto(out *ResourceQuotaSpec) {
	*out = *in
	if in.Hard != nil {
		in, out := &in.Hard, &out.Hard
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Scopes != nil {
		t := make([]ResourceQuotaScope, len(in.Scopes))
		copy(t, in.Scopes)
		out.Scopes = t
	}
	if in.ScopeSelector != nil {
		in, out := &in.ScopeSelector, &out.ScopeSelector
		*out = new(ScopeSelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ResourceQuotaSpec) DeepCopy() *ResourceQuotaSpec {
	if in == nil {
		return nil
	}
	out := new(ResourceQuotaSpec)
	in.DeepCopyInto(out)
	return out
}

type ResourceQuotaStatus struct {
	// Hard is the set of enforced hard limits for each named resource.
	// More info: https://kubernetes.io/docs/concepts/policy/resource-quotas/
	Hard map[string]apiresource.Quantity `json:"hard,omitempty"`
	// Used is the current observed total usage of the resource in the namespace.
	Used map[string]apiresource.Quantity `json:"used,omitempty"`
}

func (in *ResourceQuotaStatus) DeepCopyInto(out *ResourceQuotaStatus) {
	*out = *in
	if in.Hard != nil {
		in, out := &in.Hard, &out.Hard
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Used != nil {
		in, out := &in.Used, &out.Used
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *ResourceQuotaStatus) DeepCopy() *ResourceQuotaStatus {
	if in == nil {
		return nil
	}
	out := new(ResourceQuotaStatus)
	in.DeepCopyInto(out)
	return out
}

type ServiceSpec struct {
	// The list of ports that are exposed by this service.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	Ports []ServicePort `json:"ports"`
	// Route service traffic to pods with label keys and values matching this
	// selector. If empty or not present, the service is assumed to have an
	// external process managing its endpoints, which Kubernetes will not
	// modify. Only applies to types ClusterIP, NodePort, and LoadBalancer.
	// Ignored if type is ExternalName.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/
	Selector map[string]string `json:"selector,omitempty"`
	// clusterIP is the IP address of the service and is usually assigned
	// randomly. If an address is specified manually, is in-range (as per
	// system configuration), and is not in use, it will be allocated to the
	// service; otherwise creation of the service will fail. This field may not
	// be changed through updates unless the type field is also being changed
	// to ExternalName (which requires this field to be blank) or the type
	// field is being changed from ExternalName (in which case this field may
	// optionally be specified, as describe above).  Valid values are "None",
	// empty string (""), or a valid IP address. Setting this to "None" makes a
	// "headless service" (no virtual IP), which is useful when direct endpoint
	// connections are preferred and proxying is not required.  Only applies to
	// types ClusterIP, NodePort, and LoadBalancer. If this field is specified
	// when creating a Service of type ExternalName, creation will fail. This
	// field will be wiped when updating a Service to type ExternalName.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	ClusterIP string `json:"clusterIP,omitempty"`
	// ClusterIPs is a list of IP addresses assigned to this service, and are
	// usually assigned randomly.  If an address is specified manually, is
	// in-range (as per system configuration), and is not in use, it will be
	// allocated to the service; otherwise creation of the service will fail.
	// This field may not be changed through updates unless the type field is
	// also being changed to ExternalName (which requires this field to be
	// empty) or the type field is being changed from ExternalName (in which
	// case this field may optionally be specified, as describe above).  Valid
	// values are "None", empty string (""), or a valid IP address.  Setting
	// this to "None" makes a "headless service" (no virtual IP), which is
	// useful when direct endpoint connections are preferred and proxying is
	// not required.  Only applies to types ClusterIP, NodePort, and
	// LoadBalancer. If this field is specified when creating a Service of type
	// ExternalName, creation will fail. This field will be wiped when updating
	// a Service to type ExternalName.  If this field is not specified, it will
	// be initialized from the clusterIP field.  If this field is specified,
	// clients must ensure that clusterIPs[0] and clusterIP have the same
	// value.
	// This field may hold a maximum of two entries (dual-stack IPs, in either order).
	// These IPs must correspond to the values of the ipFamilies field. Both
	// clusterIPs and ipFamilies are governed by the ipFamilyPolicy field.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	ClusterIPs []string `json:"clusterIPs"`
	// type determines how the Service is exposed. Defaults to ClusterIP. Valid
	// options are ExternalName, ClusterIP, NodePort, and LoadBalancer.
	// "ClusterIP" allocates a cluster-internal IP address for load-balancing
	// to endpoints. Endpoints are determined by the selector or if that is not
	// specified, by manual construction of an Endpoints object or
	// EndpointSlice objects. If clusterIP is "None", no virtual IP is
	// allocated and the endpoints are published as a set of endpoints rather
	// than a virtual IP.
	// "NodePort" builds on ClusterIP and allocates a port on every node which
	// routes to the same endpoints as the clusterIP.
	// "LoadBalancer" builds on NodePort and creates an external load-balancer
	// (if supported in the current cloud) which routes to the same endpoints
	// as the clusterIP.
	// "ExternalName" aliases this service to the specified externalName.
	// Several other fields do not apply to ExternalName services.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#publishing-services-service-types
	Type ServiceType `json:"type,omitempty"`
	// externalIPs is a list of IP addresses for which nodes in the cluster
	// will also accept traffic for this service.  These IPs are not managed by
	// Kubernetes.  The user is responsible for ensuring that traffic arrives
	// at a node with this IP.  A common example is external load-balancers
	// that are not part of the Kubernetes system.
	ExternalIPs []string `json:"externalIPs"`
	// Supports "ClientIP" and "None". Used to maintain session affinity.
	// Enable client IP based session affinity.
	// Must be ClientIP or None.
	// Defaults to None.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies
	SessionAffinity ServiceAffinity `json:"sessionAffinity,omitempty"`
	// Only applies to Service Type: LoadBalancer.
	// This feature depends on whether the underlying cloud-provider supports specifying
	// the loadBalancerIP when a load balancer is created.
	// This field will be ignored if the cloud-provider does not support the feature.
	// Deprecated: This field was under-specified and its meaning varies across implementations,
	// and it cannot support dual-stack.
	// As of Kubernetes v1.24, users are encouraged to use implementation-specific annotations when available.
	// This field may be removed in a future API version.
	LoadBalancerIP string `json:"loadBalancerIP,omitempty"`
	// If specified and supported by the platform, this will restrict traffic through the cloud-provider
	// load-balancer will be restricted to the specified client IPs. This field will be ignored if the
	// cloud-provider does not support the feature."
	// More info: https://kubernetes.io/docs/tasks/access-application-cluster/create-external-load-balancer/
	LoadBalancerSourceRanges []string `json:"loadBalancerSourceRanges"`
	// externalName is the external reference that discovery mechanisms will
	// return as an alias for this service (e.g. a DNS CNAME record). No
	// proxying will be involved.  Must be a lowercase RFC-1123 hostname
	// (https://tools.ietf.org/html/rfc1123) and requires `type` to be "ExternalName".
	ExternalName string `json:"externalName,omitempty"`
	// externalTrafficPolicy describes how nodes distribute service traffic they
	// receive on one of the Service's "externally-facing" addresses (NodePorts,
	// ExternalIPs, and LoadBalancer IPs). If set to "Local", the proxy will configure
	// the service in a way that assumes that external load balancers will take care
	// of balancing the service traffic between nodes, and so each node will deliver
	// traffic only to the node-local endpoints of the service, without masquerading
	// the client source IP. (Traffic mistakenly sent to a node with no endpoints will
	// be dropped.) The default value, "Cluster", uses the standard behavior of
	// routing to all endpoints evenly (possibly modified by topology and other
	// features). Note that traffic sent to an External IP or LoadBalancer IP from
	// within the cluster will always get "Cluster" semantics, but clients sending to
	// a NodePort from within the cluster may need to take traffic policy into account
	// when picking a node.
	ExternalTrafficPolicy ServiceExternalTrafficPolicy `json:"externalTrafficPolicy,omitempty"`
	// healthCheckNodePort specifies the healthcheck nodePort for the service.
	// This only applies when type is set to LoadBalancer and
	// externalTrafficPolicy is set to Local. If a value is specified, is
	// in-range, and is not in use, it will be used.  If not specified, a value
	// will be automatically allocated.  External systems (e.g. load-balancers)
	// can use this port to determine if a given node holds endpoints for this
	// service or not.  If this field is specified when creating a Service
	// which does not need it, creation will fail. This field will be wiped
	// when updating a Service to no longer need it (e.g. changing type).
	// This field cannot be updated once set.
	HealthCheckNodePort int `json:"healthCheckNodePort,omitempty"`
	// publishNotReadyAddresses indicates that any agent which deals with endpoints for this
	// Service should disregard any indications of ready/not-ready.
	// The primary use case for setting this field is for a StatefulSet's Headless Service to
	// propagate SRV DNS records for its Pods for the purpose of peer discovery.
	// The Kubernetes controllers that generate Endpoints and EndpointSlice resources for
	// Services interpret this to mean that all endpoints are considered "ready" even if the
	// Pods themselves are not. Agents which consume only Kubernetes generated endpoints
	// through the Endpoints or EndpointSlice resources can safely assume this behavior.
	PublishNotReadyAddresses bool `json:"publishNotReadyAddresses,omitempty"`
	// sessionAffinityConfig contains the configurations of session affinity.
	SessionAffinityConfig *SessionAffinityConfig `json:"sessionAffinityConfig,omitempty"`
	// IPFamilies is a list of IP families (e.g. IPv4, IPv6) assigned to this
	// service. This field is usually assigned automatically based on cluster
	// configuration and the ipFamilyPolicy field. If this field is specified
	// manually, the requested family is available in the cluster,
	// and ipFamilyPolicy allows it, it will be used; otherwise creation of
	// the service will fail. This field is conditionally mutable: it allows
	// for adding or removing a secondary IP family, but it does not allow
	// changing the primary IP family of the Service. Valid values are "IPv4"
	// and "IPv6".  This field only applies to Services of types ClusterIP,
	// NodePort, and LoadBalancer, and does apply to "headless" services.
	// This field will be wiped when updating a Service to type ExternalName.
	// This field may hold a maximum of two entries (dual-stack families, in
	// either order).  These families must correspond to the values of the
	// clusterIPs field, if specified. Both clusterIPs and ipFamilies are
	// governed by the ipFamilyPolicy field.
	IPFamilies []IPFamily `json:"ipFamilies"`
	// IPFamilyPolicy represents the dual-stack-ness requested or required by
	// this Service. If there is no value provided, then this field will be set
	// to SingleStack. Services can be "SingleStack" (a single IP family),
	// "PreferDualStack" (two IP families on dual-stack configured clusters or
	// a single IP family on single-stack clusters), or "RequireDualStack"
	// (two IP families on dual-stack configured clusters, otherwise fail). The
	// ipFamilies and clusterIPs fields depend on the value of this field. This
	// field will be wiped when updating a service to type ExternalName.
	IPFamilyPolicy IPFamilyPolicy `json:"ipFamilyPolicy,omitempty"`
	// allocateLoadBalancerNodePorts defines if NodePorts will be automatically
	// allocated for services with type LoadBalancer.  Default is "true". It
	// may be set to "false" if the cluster load-balancer does not rely on
	// NodePorts.  If the caller requests specific NodePorts (by specifying a
	// value), those requests will be respected, regardless of this field.
	// This field may only be set for services with type LoadBalancer and will
	// be cleared if the type is changed to any other type.
	AllocateLoadBalancerNodePorts bool `json:"allocateLoadBalancerNodePorts,omitempty"`
	// loadBalancerClass is the class of the load balancer implementation this Service belongs to.
	// If specified, the value of this field must be a label-style identifier, with an optional prefix,
	// e.g. "internal-vip" or "example.com/internal-vip". Unprefixed names are reserved for end-users.
	// This field can only be set when the Service type is 'LoadBalancer'. If not set, the default load
	// balancer implementation is used, today this is typically done through the cloud provider integration,
	// but should apply for any default implementation. If set, it is assumed that a load balancer
	// implementation is watching for Services with a matching class. Any default load balancer
	// implementation (e.g. cloud providers) should ignore Services that set this field.
	// This field can only be set when creating or updating a Service to type 'LoadBalancer'.
	// Once set, it can not be changed. This field will be wiped when a service is updated to a non 'LoadBalancer' type.
	LoadBalancerClass string `json:"loadBalancerClass,omitempty"`
	// InternalTrafficPolicy describes how nodes distribute service traffic they
	// receive on the ClusterIP. If set to "Local", the proxy will assume that pods
	// only want to talk to endpoints of the service on the same node as the pod,
	// dropping the traffic if there are no local endpoints. The default value,
	// "Cluster", uses the standard behavior of routing to all endpoints evenly
	// (possibly modified by topology and other features).
	InternalTrafficPolicy ServiceInternalTrafficPolicy `json:"internalTrafficPolicy,omitempty"`
}

func (in *ServiceSpec) DeepCopyInto(out *ServiceSpec) {
	*out = *in
	if in.Ports != nil {
		l := make([]ServicePort, len(in.Ports))
		for i := range in.Ports {
			in.Ports[i].DeepCopyInto(&l[i])
		}
		out.Ports = l
	}
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.ClusterIPs != nil {
		t := make([]string, len(in.ClusterIPs))
		copy(t, in.ClusterIPs)
		out.ClusterIPs = t
	}
	if in.ExternalIPs != nil {
		t := make([]string, len(in.ExternalIPs))
		copy(t, in.ExternalIPs)
		out.ExternalIPs = t
	}
	if in.LoadBalancerSourceRanges != nil {
		t := make([]string, len(in.LoadBalancerSourceRanges))
		copy(t, in.LoadBalancerSourceRanges)
		out.LoadBalancerSourceRanges = t
	}
	if in.SessionAffinityConfig != nil {
		in, out := &in.SessionAffinityConfig, &out.SessionAffinityConfig
		*out = new(SessionAffinityConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.IPFamilies != nil {
		t := make([]IPFamily, len(in.IPFamilies))
		copy(t, in.IPFamilies)
		out.IPFamilies = t
	}
}

func (in *ServiceSpec) DeepCopy() *ServiceSpec {
	if in == nil {
		return nil
	}
	out := new(ServiceSpec)
	in.DeepCopyInto(out)
	return out
}

type ServiceStatus struct {
	// LoadBalancer contains the current status of the load-balancer,
	// if one is present.
	LoadBalancer *LoadBalancerStatus `json:"loadBalancer,omitempty"`
	// Current service state
	Conditions []metav1.Condition `json:"conditions"`
}

func (in *ServiceStatus) DeepCopyInto(out *ServiceStatus) {
	*out = *in
	if in.LoadBalancer != nil {
		in, out := &in.LoadBalancer, &out.LoadBalancer
		*out = new(LoadBalancerStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.Conditions != nil {
		l := make([]metav1.Condition, len(in.Conditions))
		for i := range in.Conditions {
			in.Conditions[i].DeepCopyInto(&l[i])
		}
		out.Conditions = l
	}
}

func (in *ServiceStatus) DeepCopy() *ServiceStatus {
	if in == nil {
		return nil
	}
	out := new(ServiceStatus)
	in.DeepCopyInto(out)
	return out
}

type LocalObjectReference struct {
	// Name of the referent.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name,omitempty"`
}

func (in *LocalObjectReference) DeepCopyInto(out *LocalObjectReference) {
	*out = *in
}

func (in *LocalObjectReference) DeepCopy() *LocalObjectReference {
	if in == nil {
		return nil
	}
	out := new(LocalObjectReference)
	in.DeepCopyInto(out)
	return out
}

type EndpointAddress struct {
	// The IP of this endpoint.
	// May not be loopback (127.0.0.0/8 or ::1), link-local (169.254.0.0/16 or fe80::/10),
	// or link-local multicast (224.0.0.0/24 or ff02::/16).
	IP string `json:"ip"`
	// The Hostname of this endpoint
	Hostname string `json:"hostname,omitempty"`
	// Optional: Node hosting this endpoint. This can be used to determine endpoints local to a node.
	NodeName string `json:"nodeName,omitempty"`
	// Reference to object providing the endpoint.
	TargetRef *ObjectReference `json:"targetRef,omitempty"`
}

func (in *EndpointAddress) DeepCopyInto(out *EndpointAddress) {
	*out = *in
	if in.TargetRef != nil {
		in, out := &in.TargetRef, &out.TargetRef
		*out = new(ObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EndpointAddress) DeepCopy() *EndpointAddress {
	if in == nil {
		return nil
	}
	out := new(EndpointAddress)
	in.DeepCopyInto(out)
	return out
}

type EndpointPort struct {
	// The name of this port.  This must match the 'name' field in the
	// corresponding ServicePort.
	// Must be a DNS_LABEL.
	// Optional only if one port is defined.
	Name string `json:"name,omitempty"`
	// The port number of the endpoint.
	Port int `json:"port"`
	// The IP protocol for this port.
	// Must be UDP, TCP, or SCTP.
	// Default is TCP.
	Protocol Protocol `json:"protocol,omitempty"`
	// The application protocol for this port.
	// This is used as a hint for implementations to offer richer behavior for protocols that they understand.
	// This field follows standard Kubernetes label syntax.
	// Valid values are either:
	// * Un-prefixed protocol names - reserved for IANA standard service names (as per
	// RFC-6335 and https://www.iana.org/assignments/service-names).
	// * Kubernetes-defined prefixed names:
	// * 'kubernetes.io/h2c' - HTTP/2 over cleartext as described in https://www.rfc-editor.org/rfc/rfc7540
	// * Other protocols should use implementation-defined prefixed names such as
	// mycompany.com/my-custom-protocol.
	AppProtocol string `json:"appProtocol,omitempty"`
}

func (in *EndpointPort) DeepCopyInto(out *EndpointPort) {
	*out = *in
}

func (in *EndpointPort) DeepCopy() *EndpointPort {
	if in == nil {
		return nil
	}
	out := new(EndpointPort)
	in.DeepCopyInto(out)
	return out
}

type LimitRangeItem struct {
	// Type of resource that this limit applies to.
	Type LimitType `json:"type"`
	// Max usage constraints on this kind by resource name.
	Max map[string]apiresource.Quantity `json:"max,omitempty"`
	// Min usage constraints on this kind by resource name.
	Min map[string]apiresource.Quantity `json:"min,omitempty"`
	// Default resource requirement limit value by resource name if resource limit is omitted.
	Default map[string]apiresource.Quantity `json:"default,omitempty"`
	// DefaultRequest is the default resource requirement request value by resource name if resource request is omitted.
	DefaultRequest map[string]apiresource.Quantity `json:"defaultRequest,omitempty"`
	// MaxLimitRequestRatio if specified, the named resource must have a request and limit that are both non-zero where limit divided by request is less than or equal to the enumerated value; this represents the max burst for the named resource.
	MaxLimitRequestRatio map[string]apiresource.Quantity `json:"maxLimitRequestRatio,omitempty"`
}

func (in *LimitRangeItem) DeepCopyInto(out *LimitRangeItem) {
	*out = *in
	if in.Max != nil {
		in, out := &in.Max, &out.Max
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Min != nil {
		in, out := &in.Min, &out.Min
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Default != nil {
		in, out := &in.Default, &out.Default
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.DefaultRequest != nil {
		in, out := &in.DefaultRequest, &out.DefaultRequest
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.MaxLimitRequestRatio != nil {
		in, out := &in.MaxLimitRequestRatio, &out.MaxLimitRequestRatio
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *LimitRangeItem) DeepCopy() *LimitRangeItem {
	if in == nil {
		return nil
	}
	out := new(LimitRangeItem)
	in.DeepCopyInto(out)
	return out
}

type NamespaceCondition struct {
	// Type of namespace controller condition.
	Type NamespaceConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status             ConditionStatus `json:"status"`
	LastTransitionTime *metav1.Time    `json:"lastTransitionTime,omitempty"`
	Reason             string          `json:"reason,omitempty"`
	Message            string          `json:"message,omitempty"`
}

func (in *NamespaceCondition) DeepCopyInto(out *NamespaceCondition) {
	*out = *in
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NamespaceCondition) DeepCopy() *NamespaceCondition {
	if in == nil {
		return nil
	}
	out := new(NamespaceCondition)
	in.DeepCopyInto(out)
	return out
}

type Taint struct {
	// Required. The taint key to be applied to a node.
	Key string `json:"key"`
	// The taint value corresponding to the taint key.
	Value string `json:"value,omitempty"`
	// Required. The effect of the taint on pods
	// that do not tolerate the taint.
	// Valid effects are NoSchedule, PreferNoSchedule and NoExecute.
	Effect TaintEffect `json:"effect"`
	// TimeAdded represents the time at which the taint was added.
	// It is only written for NoExecute taints.
	TimeAdded *metav1.Time `json:"timeAdded,omitempty"`
}

func (in *Taint) DeepCopyInto(out *Taint) {
	*out = *in
	if in.TimeAdded != nil {
		in, out := &in.TimeAdded, &out.TimeAdded
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Taint) DeepCopy() *Taint {
	if in == nil {
		return nil
	}
	out := new(Taint)
	in.DeepCopyInto(out)
	return out
}

type NodeConfigSource struct {
	// ConfigMap is a reference to a Node's ConfigMap
	ConfigMap *ConfigMapNodeConfigSource `json:"configMap,omitempty"`
}

func (in *NodeConfigSource) DeepCopyInto(out *NodeConfigSource) {
	*out = *in
	if in.ConfigMap != nil {
		in, out := &in.ConfigMap, &out.ConfigMap
		*out = new(ConfigMapNodeConfigSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NodeConfigSource) DeepCopy() *NodeConfigSource {
	if in == nil {
		return nil
	}
	out := new(NodeConfigSource)
	in.DeepCopyInto(out)
	return out
}

type NodeCondition struct {
	// Type of node condition.
	Type NodeConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// Last time we got an update on a given condition.
	LastHeartbeatTime *metav1.Time `json:"lastHeartbeatTime,omitempty"`
	// Last time the condition transit from one status to another.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// (brief) reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

func (in *NodeCondition) DeepCopyInto(out *NodeCondition) {
	*out = *in
	if in.LastHeartbeatTime != nil {
		in, out := &in.LastHeartbeatTime, &out.LastHeartbeatTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NodeCondition) DeepCopy() *NodeCondition {
	if in == nil {
		return nil
	}
	out := new(NodeCondition)
	in.DeepCopyInto(out)
	return out
}

type NodeAddress struct {
	// Node address type, one of Hostname, ExternalIP or InternalIP.
	Type NodeAddressType `json:"type"`
	// The node address.
	Address string `json:"address"`
}

func (in *NodeAddress) DeepCopyInto(out *NodeAddress) {
	*out = *in
}

func (in *NodeAddress) DeepCopy() *NodeAddress {
	if in == nil {
		return nil
	}
	out := new(NodeAddress)
	in.DeepCopyInto(out)
	return out
}

type NodeDaemonEndpoints struct {
	// Endpoint on which Kubelet is listening.
	KubeletEndpoint *DaemonEndpoint `json:"kubeletEndpoint,omitempty"`
}

func (in *NodeDaemonEndpoints) DeepCopyInto(out *NodeDaemonEndpoints) {
	*out = *in
	if in.KubeletEndpoint != nil {
		in, out := &in.KubeletEndpoint, &out.KubeletEndpoint
		*out = new(DaemonEndpoint)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NodeDaemonEndpoints) DeepCopy() *NodeDaemonEndpoints {
	if in == nil {
		return nil
	}
	out := new(NodeDaemonEndpoints)
	in.DeepCopyInto(out)
	return out
}

type NodeSystemInfo struct {
	// MachineID reported by the node. For unique machine identification
	// in the cluster this field is preferred. Learn more from man(5)
	// machine-id: http://man7.org/linux/man-pages/man5/machine-id.5.html
	MachineID string `json:"machineID"`
	// SystemUUID reported by the node. For unique machine identification
	// MachineID is preferred. This field is specific to Red Hat hosts
	// https://access.redhat.com/documentation/en-us/red_hat_subscription_management/1/html/rhsm/uuid
	SystemUUID string `json:"systemUUID"`
	// Boot ID reported by the node.
	BootID string `json:"bootID"`
	// Kernel Version reported by the node from 'uname -r' (e.g. 3.16.0-0.bpo.4-amd64).
	KernelVersion string `json:"kernelVersion"`
	// OS Image reported by the node from /etc/os-release (e.g. Debian GNU/Linux 7 (wheezy)).
	OSImage string `json:"osImage"`
	// ContainerRuntime Version reported by the node through runtime remote API (e.g. containerd://1.4.2).
	ContainerRuntimeVersion string `json:"containerRuntimeVersion"`
	// Kubelet Version reported by the node.
	KubeletVersion string `json:"kubeletVersion"`
	// KubeProxy Version reported by the node.
	KubeProxyVersion string `json:"kubeProxyVersion"`
	// The Operating System reported by the node
	OperatingSystem string `json:"operatingSystem"`
	// The Architecture reported by the node
	Architecture string `json:"architecture"`
}

func (in *NodeSystemInfo) DeepCopyInto(out *NodeSystemInfo) {
	*out = *in
}

func (in *NodeSystemInfo) DeepCopy() *NodeSystemInfo {
	if in == nil {
		return nil
	}
	out := new(NodeSystemInfo)
	in.DeepCopyInto(out)
	return out
}

type ContainerImage struct {
	// Names by which this image is known.
	// e.g. ["kubernetes.example/hyperkube:v1.0.7", "cloud-vendor.registry.example/cloud-vendor/hyperkube:v1.0.7"]
	Names []string `json:"names"`
	// The size of the image in bytes.
	SizeBytes int64 `json:"sizeBytes,omitempty"`
}

func (in *ContainerImage) DeepCopyInto(out *ContainerImage) {
	*out = *in
	if in.Names != nil {
		t := make([]string, len(in.Names))
		copy(t, in.Names)
		out.Names = t
	}
}

func (in *ContainerImage) DeepCopy() *ContainerImage {
	if in == nil {
		return nil
	}
	out := new(ContainerImage)
	in.DeepCopyInto(out)
	return out
}

type AttachedVolume struct {
	// Name of the attached volume
	Name string `json:"name"`
	// DevicePath represents the device path where the volume should be available
	DevicePath string `json:"devicePath"`
}

func (in *AttachedVolume) DeepCopyInto(out *AttachedVolume) {
	*out = *in
}

func (in *AttachedVolume) DeepCopy() *AttachedVolume {
	if in == nil {
		return nil
	}
	out := new(AttachedVolume)
	in.DeepCopyInto(out)
	return out
}

type NodeConfigStatus struct {
	// Assigned reports the checkpointed config the node will try to use.
	// When Node.Spec.ConfigSource is updated, the node checkpoints the associated
	// config payload to local disk, along with a record indicating intended
	// config. The node refers to this record to choose its config checkpoint, and
	// reports this record in Assigned. Assigned only updates in the status after
	// the record has been checkpointed to disk. When the Kubelet is restarted,
	// it tries to make the Assigned config the Active config by loading and
	// validating the checkpointed payload identified by Assigned.
	Assigned *NodeConfigSource `json:"assigned,omitempty"`
	// Active reports the checkpointed config the node is actively using.
	// Active will represent either the current version of the Assigned config,
	// or the current LastKnownGood config, depending on whether attempting to use the
	// Assigned config results in an error.
	Active *NodeConfigSource `json:"active,omitempty"`
	// LastKnownGood reports the checkpointed config the node will fall back to
	// when it encounters an error attempting to use the Assigned config.
	// The Assigned config becomes the LastKnownGood config when the node determines
	// that the Assigned config is stable and correct.
	// This is currently implemented as a 10-minute soak period starting when the local
	// record of Assigned config is updated. If the Assigned config is Active at the end
	// of this period, it becomes the LastKnownGood. Note that if Spec.ConfigSource is
	// reset to nil (use local defaults), the LastKnownGood is also immediately reset to nil,
	// because the local default config is always assumed good.
	// You should not make assumptions about the node's method of determining config stability
	// and correctness, as this may change or become configurable in the future.
	LastKnownGood *NodeConfigSource `json:"lastKnownGood,omitempty"`
	// Error describes any problems reconciling the Spec.ConfigSource to the Active config.
	// Errors may occur, for example, attempting to checkpoint Spec.ConfigSource to the local Assigned
	// record, attempting to checkpoint the payload associated with Spec.ConfigSource, attempting
	// to load or validate the Assigned config, etc.
	// Errors may occur at different points while syncing config. Earlier errors (e.g. download or
	// checkpointing errors) will not result in a rollback to LastKnownGood, and may resolve across
	// Kubelet retries. Later errors (e.g. loading or validating a checkpointed config) will result in
	// a rollback to LastKnownGood. In the latter case, it is usually possible to resolve the error
	// by fixing the config assigned in Spec.ConfigSource.
	// You can find additional information for debugging by searching the error message in the Kubelet log.
	// Error is a human-readable description of the error state; machines can check whether or not Error
	// is empty, but should not rely on the stability of the Error text across Kubelet versions.
	Error string `json:"error,omitempty"`
}

func (in *NodeConfigStatus) DeepCopyInto(out *NodeConfigStatus) {
	*out = *in
	if in.Assigned != nil {
		in, out := &in.Assigned, &out.Assigned
		*out = new(NodeConfigSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Active != nil {
		in, out := &in.Active, &out.Active
		*out = new(NodeConfigSource)
		(*in).DeepCopyInto(*out)
	}
	if in.LastKnownGood != nil {
		in, out := &in.LastKnownGood, &out.LastKnownGood
		*out = new(NodeConfigSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *NodeConfigStatus) DeepCopy() *NodeConfigStatus {
	if in == nil {
		return nil
	}
	out := new(NodeConfigStatus)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeSource struct {
	// gcePersistentDisk represents a GCE Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod. Provisioned by an admin.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
	GCEPersistentDisk *GCEPersistentDiskVolumeSource `json:"gcePersistentDisk,omitempty"`
	// awsElasticBlockStore represents an AWS Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
	AWSElasticBlockStore *AWSElasticBlockStoreVolumeSource `json:"awsElasticBlockStore,omitempty"`
	// hostPath represents a directory on the host.
	// Provisioned by a developer or tester.
	// This is useful for single-node development and testing only!
	// On-host storage is not supported in any way and WILL NOT WORK in a multi-node cluster.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`
	// glusterfs represents a Glusterfs volume that is attached to a host and
	// exposed to the pod. Provisioned by an admin.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md
	Glusterfs *GlusterfsPersistentVolumeSource `json:"glusterfs,omitempty"`
	// nfs represents an NFS mount on the host. Provisioned by an admin.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
	NFS *NFSVolumeSource `json:"nfs,omitempty"`
	// rbd represents a Rados Block Device mount on the host that shares a pod's lifetime.
	// More info: https://examples.k8s.io/volumes/rbd/README.md
	RBD *RBDPersistentVolumeSource `json:"rbd,omitempty"`
	// iscsi represents an ISCSI Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod. Provisioned by an admin.
	ISCSI *ISCSIPersistentVolumeSource `json:"iscsi,omitempty"`
	// cinder represents a cinder volume attached and mounted on kubelets host machine.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	Cinder *CinderPersistentVolumeSource `json:"cinder,omitempty"`
	// cephFS represents a Ceph FS mount on the host that shares a pod's lifetime
	CephFS *CephFSPersistentVolumeSource `json:"cephfs,omitempty"`
	// fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod.
	FC *FCVolumeSource `json:"fc,omitempty"`
	// flocker represents a Flocker volume attached to a kubelet's host machine and exposed to the pod for its usage. This depends on the Flocker control service being running
	Flocker *FlockerVolumeSource `json:"flocker,omitempty"`
	// flexVolume represents a generic volume resource that is
	// provisioned/attached using an exec based plugin.
	FlexVolume *FlexPersistentVolumeSource `json:"flexVolume,omitempty"`
	// azureFile represents an Azure File Service mount on the host and bind mount to the pod.
	AzureFile *AzureFilePersistentVolumeSource `json:"azureFile,omitempty"`
	// vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine
	VsphereVolume *VsphereVirtualDiskVolumeSource `json:"vsphereVolume,omitempty"`
	// quobyte represents a Quobyte mount on the host that shares a pod's lifetime
	Quobyte *QuobyteVolumeSource `json:"quobyte,omitempty"`
	// azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod.
	AzureDisk *AzureDiskVolumeSource `json:"azureDisk,omitempty"`
	// photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine
	PhotonPersistentDisk *PhotonPersistentDiskVolumeSource `json:"photonPersistentDisk,omitempty"`
	// portworxVolume represents a portworx volume attached and mounted on kubelets host machine
	PortworxVolume *PortworxVolumeSource `json:"portworxVolume,omitempty"`
	// scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes.
	ScaleIO *ScaleIOPersistentVolumeSource `json:"scaleIO,omitempty"`
	// local represents directly-attached storage with node affinity
	Local *LocalVolumeSource `json:"local,omitempty"`
	// storageOS represents a StorageOS volume that is attached to the kubelet's host machine and mounted into the pod
	// More info: https://examples.k8s.io/volumes/storageos/README.md
	StorageOS *StorageOSPersistentVolumeSource `json:"storageos,omitempty"`
	// csi represents storage that is handled by an external CSI driver (Beta feature).
	CSI *CSIPersistentVolumeSource `json:"csi,omitempty"`
}

func (in *PersistentVolumeSource) DeepCopyInto(out *PersistentVolumeSource) {
	*out = *in
	if in.GCEPersistentDisk != nil {
		in, out := &in.GCEPersistentDisk, &out.GCEPersistentDisk
		*out = new(GCEPersistentDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.AWSElasticBlockStore != nil {
		in, out := &in.AWSElasticBlockStore, &out.AWSElasticBlockStore
		*out = new(AWSElasticBlockStoreVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.HostPath != nil {
		in, out := &in.HostPath, &out.HostPath
		*out = new(HostPathVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Glusterfs != nil {
		in, out := &in.Glusterfs, &out.Glusterfs
		*out = new(GlusterfsPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.NFS != nil {
		in, out := &in.NFS, &out.NFS
		*out = new(NFSVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.RBD != nil {
		in, out := &in.RBD, &out.RBD
		*out = new(RBDPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.ISCSI != nil {
		in, out := &in.ISCSI, &out.ISCSI
		*out = new(ISCSIPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Cinder != nil {
		in, out := &in.Cinder, &out.Cinder
		*out = new(CinderPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.CephFS != nil {
		in, out := &in.CephFS, &out.CephFS
		*out = new(CephFSPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.FC != nil {
		in, out := &in.FC, &out.FC
		*out = new(FCVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Flocker != nil {
		in, out := &in.Flocker, &out.Flocker
		*out = new(FlockerVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.FlexVolume != nil {
		in, out := &in.FlexVolume, &out.FlexVolume
		*out = new(FlexPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.AzureFile != nil {
		in, out := &in.AzureFile, &out.AzureFile
		*out = new(AzureFilePersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.VsphereVolume != nil {
		in, out := &in.VsphereVolume, &out.VsphereVolume
		*out = new(VsphereVirtualDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Quobyte != nil {
		in, out := &in.Quobyte, &out.Quobyte
		*out = new(QuobyteVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.AzureDisk != nil {
		in, out := &in.AzureDisk, &out.AzureDisk
		*out = new(AzureDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.PhotonPersistentDisk != nil {
		in, out := &in.PhotonPersistentDisk, &out.PhotonPersistentDisk
		*out = new(PhotonPersistentDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.PortworxVolume != nil {
		in, out := &in.PortworxVolume, &out.PortworxVolume
		*out = new(PortworxVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.ScaleIO != nil {
		in, out := &in.ScaleIO, &out.ScaleIO
		*out = new(ScaleIOPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Local != nil {
		in, out := &in.Local, &out.Local
		*out = new(LocalVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.StorageOS != nil {
		in, out := &in.StorageOS, &out.StorageOS
		*out = new(StorageOSPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.CSI != nil {
		in, out := &in.CSI, &out.CSI
		*out = new(CSIPersistentVolumeSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentVolumeSource) DeepCopy() *PersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type VolumeNodeAffinity struct {
	// required specifies hard node constraints that must be met.
	Required *NodeSelector `json:"required,omitempty"`
}

func (in *VolumeNodeAffinity) DeepCopyInto(out *VolumeNodeAffinity) {
	*out = *in
	if in.Required != nil {
		in, out := &in.Required, &out.Required
		*out = new(NodeSelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *VolumeNodeAffinity) DeepCopy() *VolumeNodeAffinity {
	if in == nil {
		return nil
	}
	out := new(VolumeNodeAffinity)
	in.DeepCopyInto(out)
	return out
}

type ResourceRequirements struct {
	// Limits describes the maximum amount of compute resources allowed.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Limits map[string]apiresource.Quantity `json:"limits,omitempty"`
	// Requests describes the minimum amount of compute resources required.
	// If Requests is omitted for a container, it defaults to Limits if that is explicitly specified,
	// otherwise to an implementation-defined value. Requests cannot exceed Limits.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Requests map[string]apiresource.Quantity `json:"requests,omitempty"`
	// Claims lists the names of resources, defined in spec.resourceClaims,
	// that are used by this container.
	// This is an alpha field and requires enabling the
	// DynamicResourceAllocation feature gate.
	// This field is immutable. It can only be set for containers.
	Claims []ResourceClaim `json:"claims"`
}

func (in *ResourceRequirements) DeepCopyInto(out *ResourceRequirements) {
	*out = *in
	if in.Limits != nil {
		in, out := &in.Limits, &out.Limits
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Requests != nil {
		in, out := &in.Requests, &out.Requests
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Claims != nil {
		l := make([]ResourceClaim, len(in.Claims))
		for i := range in.Claims {
			in.Claims[i].DeepCopyInto(&l[i])
		}
		out.Claims = l
	}
}

func (in *ResourceRequirements) DeepCopy() *ResourceRequirements {
	if in == nil {
		return nil
	}
	out := new(ResourceRequirements)
	in.DeepCopyInto(out)
	return out
}

type TypedLocalObjectReference struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind is the type of resource being referenced
	Kind string `json:"kind"`
	// Name is the name of resource being referenced
	Name string `json:"name"`
}

func (in *TypedLocalObjectReference) DeepCopyInto(out *TypedLocalObjectReference) {
	*out = *in
}

func (in *TypedLocalObjectReference) DeepCopy() *TypedLocalObjectReference {
	if in == nil {
		return nil
	}
	out := new(TypedLocalObjectReference)
	in.DeepCopyInto(out)
	return out
}

type TypedObjectReference struct {
	// APIGroup is the group for the resource being referenced.
	// If APIGroup is not specified, the specified Kind must be in the core API group.
	// For any other third-party types, APIGroup is required.
	APIGroup string `json:"apiGroup,omitempty"`
	// Kind is the type of resource being referenced
	Kind string `json:"kind"`
	// Name is the name of resource being referenced
	Name string `json:"name"`
	// Namespace is the namespace of resource being referenced
	// Note that when a namespace is specified, a gateway.networking.k8s.io/ReferenceGrant object is required in the referent namespace to allow that namespace's owner to accept the reference. See the ReferenceGrant documentation for details.
	// (Alpha) This field requires the CrossNamespaceVolumeDataSource feature gate to be enabled.
	Namespace string `json:"namespace,omitempty"`
}

func (in *TypedObjectReference) DeepCopyInto(out *TypedObjectReference) {
	*out = *in
}

func (in *TypedObjectReference) DeepCopy() *TypedObjectReference {
	if in == nil {
		return nil
	}
	out := new(TypedObjectReference)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeClaimCondition struct {
	Type   PersistentVolumeClaimConditionType `json:"type"`
	Status ConditionStatus                    `json:"status"`
	// lastProbeTime is the time we probed the condition.
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`
	// lastTransitionTime is the time the condition transitioned from one status to another.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// reason is a unique, this should be a short, machine understandable string that gives the reason
	// for condition's last transition. If it reports "ResizeStarted" that means the underlying
	// persistent volume is being resized.
	Reason string `json:"reason,omitempty"`
	// message is the human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

func (in *PersistentVolumeClaimCondition) DeepCopyInto(out *PersistentVolumeClaimCondition) {
	*out = *in
	if in.LastProbeTime != nil {
		in, out := &in.LastProbeTime, &out.LastProbeTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentVolumeClaimCondition) DeepCopy() *PersistentVolumeClaimCondition {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimCondition)
	in.DeepCopyInto(out)
	return out
}

type Volume struct {
	// name of the volume.
	// Must be a DNS_LABEL and unique within the pod.
	// More info: https://kubernetes.io/docs/concepts/overview/working-with-objects/names/#names
	Name string `json:"name"`
	// volumeSource represents the location and type of the mounted volume.
	// If not specified, the Volume is implied to be an EmptyDir.
	// This implied behavior is deprecated and will be removed in a future version.
	VolumeSource `json:",inline"`
}

func (in *Volume) DeepCopyInto(out *Volume) {
	*out = *in
	out.VolumeSource = in.VolumeSource
}

func (in *Volume) DeepCopy() *Volume {
	if in == nil {
		return nil
	}
	out := new(Volume)
	in.DeepCopyInto(out)
	return out
}

type Container struct {
	// Name of the container specified as a DNS_LABEL.
	// Each container in a pod must have a unique name (DNS_LABEL).
	// Cannot be updated.
	Name string `json:"name"`
	// Container image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	// This field is optional to allow higher level config management to default or override
	// container images in workload controllers like Deployments and StatefulSets.
	Image string `json:"image,omitempty"`
	// Entrypoint array. Not executed within a shell.
	// The container image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	Command []string `json:"command"`
	// Arguments to the entrypoint.
	// The container image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	Args []string `json:"args"`
	// Container's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	WorkingDir string `json:"workingDir,omitempty"`
	// List of ports to expose from the container. Not specifying a port here
	// DOES NOT prevent that port from being exposed. Any port which is
	// listening on the default "0.0.0.0" address inside a container will be
	// accessible from the network.
	// Modifying this array with strategic merge patch may corrupt the data.
	// For more information See https://github.com/kubernetes/kubernetes/issues/108255.
	// Cannot be updated.
	Ports []ContainerPort `json:"ports"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	EnvFrom []EnvFromSource `json:"envFrom"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	Env []EnvVar `json:"env"`
	// Compute Resources required by this container.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// Resources resize policy for the container.
	ResizePolicy []ContainerResizePolicy `json:"resizePolicy"`
	// Pod volumes to mount into the container's filesystem.
	// Cannot be updated.
	VolumeMounts []VolumeMount `json:"volumeMounts"`
	// volumeDevices is the list of block devices to be used by the container.
	VolumeDevices []VolumeDevice `json:"volumeDevices"`
	// Periodic probe of container liveness.
	// Container will be restarted if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// Periodic probe of container service readiness.
	// Container will be removed from service endpoints if the probe fails.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	// StartupProbe indicates that the Pod has successfully initialized.
	// If specified, no other probes are executed until this completes successfully.
	// If this probe fails, the Pod will be restarted, just as if the livenessProbe failed.
	// This can be used to provide different probe parameters at the beginning of a Pod's lifecycle,
	// when it might take a long time to load data or warm a cache, than during steady-state operation.
	// This cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	StartupProbe *Probe `json:"startupProbe,omitempty"`
	// Actions that the management system should take in response to container lifecycle events.
	// Cannot be updated.
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
	// Optional: Path at which the file to which the container's termination message
	// will be written is mounted into the container's filesystem.
	// Message written is intended to be brief final status, such as an assertion failure message.
	// Will be truncated by the node if greater than 4096 bytes. The total message length across
	// all containers will be limited to 12kb.
	// Defaults to /dev/termination-log.
	// Cannot be updated.
	TerminationMessagePath string `json:"terminationMessagePath,omitempty"`
	// Indicate how the termination message should be populated. File will use the contents of
	// terminationMessagePath to populate the container status message on both success and failure.
	// FallbackToLogsOnError will use the last chunk of container log output if the termination
	// message file is empty and the container exited with an error.
	// The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
	// Defaults to File.
	// Cannot be updated.
	TerminationMessagePolicy TerminationMessagePolicy `json:"terminationMessagePolicy,omitempty"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	ImagePullPolicy PullPolicy `json:"imagePullPolicy,omitempty"`
	// SecurityContext defines the security options the container should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	// More info: https://kubernetes.io/docs/tasks/configure-pod-container/security-context/
	SecurityContext *SecurityContext `json:"securityContext,omitempty"`
	// Whether this container should allocate a buffer for stdin in the container runtime. If this
	// is not set, reads from stdin in the container will always result in EOF.
	// Default is false.
	Stdin bool `json:"stdin,omitempty"`
	// Whether the container runtime should close the stdin channel after it has been opened by
	// a single attach. When stdin is true the stdin stream will remain open across multiple attach
	// sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
	// first client attaches to stdin, and then remains open and accepts data until the client disconnects,
	// at which time stdin is closed and remains closed until the container is restarted. If this
	// flag is false, a container processes that reads from stdin will never receive an EOF.
	// Default is false
	StdinOnce bool `json:"stdinOnce,omitempty"`
	// Whether this container should allocate a TTY for itself, also requires 'stdin' to be true.
	// Default is false.
	TTY bool `json:"tty,omitempty"`
}

func (in *Container) DeepCopyInto(out *Container) {
	*out = *in
	if in.Command != nil {
		t := make([]string, len(in.Command))
		copy(t, in.Command)
		out.Command = t
	}
	if in.Args != nil {
		t := make([]string, len(in.Args))
		copy(t, in.Args)
		out.Args = t
	}
	if in.Ports != nil {
		l := make([]ContainerPort, len(in.Ports))
		for i := range in.Ports {
			in.Ports[i].DeepCopyInto(&l[i])
		}
		out.Ports = l
	}
	if in.EnvFrom != nil {
		l := make([]EnvFromSource, len(in.EnvFrom))
		for i := range in.EnvFrom {
			in.EnvFrom[i].DeepCopyInto(&l[i])
		}
		out.EnvFrom = l
	}
	if in.Env != nil {
		l := make([]EnvVar, len(in.Env))
		for i := range in.Env {
			in.Env[i].DeepCopyInto(&l[i])
		}
		out.Env = l
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.ResizePolicy != nil {
		l := make([]ContainerResizePolicy, len(in.ResizePolicy))
		for i := range in.ResizePolicy {
			in.ResizePolicy[i].DeepCopyInto(&l[i])
		}
		out.ResizePolicy = l
	}
	if in.VolumeMounts != nil {
		l := make([]VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			in.VolumeMounts[i].DeepCopyInto(&l[i])
		}
		out.VolumeMounts = l
	}
	if in.VolumeDevices != nil {
		l := make([]VolumeDevice, len(in.VolumeDevices))
		for i := range in.VolumeDevices {
			in.VolumeDevices[i].DeepCopyInto(&l[i])
		}
		out.VolumeDevices = l
	}
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.StartupProbe != nil {
		in, out := &in.StartupProbe, &out.StartupProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.Lifecycle != nil {
		in, out := &in.Lifecycle, &out.Lifecycle
		*out = new(Lifecycle)
		(*in).DeepCopyInto(*out)
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(SecurityContext)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Container) DeepCopy() *Container {
	if in == nil {
		return nil
	}
	out := new(Container)
	in.DeepCopyInto(out)
	return out
}

type EphemeralContainer struct {
	// Ephemeral containers have all of the fields of Container, plus additional fields
	// specific to ephemeral containers. Fields in common with Container are in the
	// following inlined struct so than an EphemeralContainer may easily be converted
	// to a Container.
	EphemeralContainerCommon `json:",inline"`
	// If set, the name of the container from PodSpec that this ephemeral container targets.
	// The ephemeral container will be run in the namespaces (IPC, PID, etc) of this container.
	// If not set then the ephemeral container uses the namespaces configured in the Pod spec.
	// The container runtime must implement support for this feature. If the runtime does not
	// support namespace targeting then the result of setting this field is undefined.
	TargetContainerName string `json:"targetContainerName,omitempty"`
}

func (in *EphemeralContainer) DeepCopyInto(out *EphemeralContainer) {
	*out = *in
	out.EphemeralContainerCommon = in.EphemeralContainerCommon
}

func (in *EphemeralContainer) DeepCopy() *EphemeralContainer {
	if in == nil {
		return nil
	}
	out := new(EphemeralContainer)
	in.DeepCopyInto(out)
	return out
}

type PodSecurityContext struct {
	// The SELinux context to be applied to all containers.
	// If unspecified, the container runtime will allocate a random SELinux context for each
	// container.  May also be set in SecurityContext.  If set in
	// both SecurityContext and PodSecurityContext, the value specified in SecurityContext
	// takes precedence for that container.
	// Note that this field cannot be set when spec.os.name is windows.
	SELinuxOptions *SELinuxOptions `json:"seLinuxOptions,omitempty"`
	// The Windows specific settings applied to all containers.
	// If unspecified, the options within a container's SecurityContext will be used.
	// If set in both SecurityContext and PodSecurityContext, the value specified in SecurityContext takes precedence.
	// Note that this field cannot be set when spec.os.name is linux.
	WindowsOptions *WindowsSecurityContextOptions `json:"windowsOptions,omitempty"`
	// The UID to run the entrypoint of the container process.
	// Defaults to user specified in image metadata if unspecified.
	// May also be set in SecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence
	// for that container.
	// Note that this field cannot be set when spec.os.name is windows.
	RunAsUser int64 `json:"runAsUser,omitempty"`
	// The GID to run the entrypoint of the container process.
	// Uses runtime default if unset.
	// May also be set in SecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence
	// for that container.
	// Note that this field cannot be set when spec.os.name is windows.
	RunAsGroup int64 `json:"runAsGroup,omitempty"`
	// Indicates that the container must run as a non-root user.
	// If true, the Kubelet will validate the image at runtime to ensure that it
	// does not run as UID 0 (root) and fail to start the container if it does.
	// If unset or false, no such validation will be performed.
	// May also be set in SecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence.
	RunAsNonRoot bool `json:"runAsNonRoot,omitempty"`
	// A list of groups applied to the first process run in each container, in addition
	// to the container's primary GID, the fsGroup (if specified), and group memberships
	// defined in the container image for the uid of the container process. If unspecified,
	// no additional groups are added to any container. Note that group memberships
	// defined in the container image for the uid of the container process are still effective,
	// even if they are not included in this list.
	// Note that this field cannot be set when spec.os.name is windows.
	SupplementalGroups []int64 `json:"supplementalGroups"`
	// A special supplemental group that applies to all containers in a pod.
	// Some volume types allow the Kubelet to change the ownership of that volume
	// to be owned by the pod:
	// 1. The owning GID will be the FSGroup
	// 2. The setgid bit is set (new files created in the volume will be owned by FSGroup)
	// 3. The permission bits are OR'd with rw-rw----
	// If unset, the Kubelet will not modify the ownership and permissions of any volume.
	// Note that this field cannot be set when spec.os.name is windows.
	FSGroup int64 `json:"fsGroup,omitempty"`
	// Sysctls hold a list of namespaced sysctls used for the pod. Pods with unsupported
	// sysctls (by the container runtime) might fail to launch.
	// Note that this field cannot be set when spec.os.name is windows.
	Sysctls []Sysctl `json:"sysctls"`
	// fsGroupChangePolicy defines behavior of changing ownership and permission of the volume
	// before being exposed inside Pod. This field will only apply to
	// volume types which support fsGroup based ownership(and permissions).
	// It will have no effect on ephemeral volume types such as: secret, configmaps
	// and emptydir.
	// Valid values are "OnRootMismatch" and "Always". If not specified, "Always" is used.
	// Note that this field cannot be set when spec.os.name is windows.
	FSGroupChangePolicy PodFSGroupChangePolicy `json:"fsGroupChangePolicy,omitempty"`
	// The seccomp options to use by the containers in this pod.
	// Note that this field cannot be set when spec.os.name is windows.
	SeccompProfile *SeccompProfile `json:"seccompProfile,omitempty"`
}

func (in *PodSecurityContext) DeepCopyInto(out *PodSecurityContext) {
	*out = *in
	if in.SELinuxOptions != nil {
		in, out := &in.SELinuxOptions, &out.SELinuxOptions
		*out = new(SELinuxOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.WindowsOptions != nil {
		in, out := &in.WindowsOptions, &out.WindowsOptions
		*out = new(WindowsSecurityContextOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.SupplementalGroups != nil {
		t := make([]int64, len(in.SupplementalGroups))
		copy(t, in.SupplementalGroups)
		out.SupplementalGroups = t
	}
	if in.Sysctls != nil {
		l := make([]Sysctl, len(in.Sysctls))
		for i := range in.Sysctls {
			in.Sysctls[i].DeepCopyInto(&l[i])
		}
		out.Sysctls = l
	}
	if in.SeccompProfile != nil {
		in, out := &in.SeccompProfile, &out.SeccompProfile
		*out = new(SeccompProfile)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodSecurityContext) DeepCopy() *PodSecurityContext {
	if in == nil {
		return nil
	}
	out := new(PodSecurityContext)
	in.DeepCopyInto(out)
	return out
}

type Affinity struct {
	// Describes node affinity scheduling rules for the pod.
	NodeAffinity *NodeAffinity `json:"nodeAffinity,omitempty"`
	// Describes pod affinity scheduling rules (e.g. co-locate this pod in the same node, zone, etc. as some other pod(s)).
	PodAffinity *PodAffinity `json:"podAffinity,omitempty"`
	// Describes pod anti-affinity scheduling rules (e.g. avoid putting this pod in the same node, zone, etc. as some other pod(s)).
	PodAntiAffinity *PodAntiAffinity `json:"podAntiAffinity,omitempty"`
}

func (in *Affinity) DeepCopyInto(out *Affinity) {
	*out = *in
	if in.NodeAffinity != nil {
		in, out := &in.NodeAffinity, &out.NodeAffinity
		*out = new(NodeAffinity)
		(*in).DeepCopyInto(*out)
	}
	if in.PodAffinity != nil {
		in, out := &in.PodAffinity, &out.PodAffinity
		*out = new(PodAffinity)
		(*in).DeepCopyInto(*out)
	}
	if in.PodAntiAffinity != nil {
		in, out := &in.PodAntiAffinity, &out.PodAntiAffinity
		*out = new(PodAntiAffinity)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Affinity) DeepCopy() *Affinity {
	if in == nil {
		return nil
	}
	out := new(Affinity)
	in.DeepCopyInto(out)
	return out
}

type Toleration struct {
	// Key is the taint key that the toleration applies to. Empty means match all taint keys.
	// If the key is empty, operator must be Exists; this combination means to match all values and all keys.
	Key string `json:"key,omitempty"`
	// Operator represents a key's relationship to the value.
	// Valid operators are Exists and Equal. Defaults to Equal.
	// Exists is equivalent to wildcard for value, so that a pod can
	// tolerate all taints of a particular category.
	Operator TolerationOperator `json:"operator,omitempty"`
	// Value is the taint value the toleration matches to.
	// If the operator is Exists, the value should be empty, otherwise just a regular string.
	Value string `json:"value,omitempty"`
	// Effect indicates the taint effect to match. Empty means match all taint effects.
	// When specified, allowed values are NoSchedule, PreferNoSchedule and NoExecute.
	Effect TaintEffect `json:"effect,omitempty"`
	// TolerationSeconds represents the period of time the toleration (which must be
	// of effect NoExecute, otherwise this field is ignored) tolerates the taint. By default,
	// it is not set, which means tolerate the taint forever (do not evict). Zero and
	// negative values will be treated as 0 (evict immediately) by the system.
	TolerationSeconds int64 `json:"tolerationSeconds,omitempty"`
}

func (in *Toleration) DeepCopyInto(out *Toleration) {
	*out = *in
}

func (in *Toleration) DeepCopy() *Toleration {
	if in == nil {
		return nil
	}
	out := new(Toleration)
	in.DeepCopyInto(out)
	return out
}

type HostAlias struct {
	// IP address of the host file entry.
	IP string `json:"ip,omitempty"`
	// Hostnames for the above IP address.
	Hostnames []string `json:"hostnames"`
}

func (in *HostAlias) DeepCopyInto(out *HostAlias) {
	*out = *in
	if in.Hostnames != nil {
		t := make([]string, len(in.Hostnames))
		copy(t, in.Hostnames)
		out.Hostnames = t
	}
}

func (in *HostAlias) DeepCopy() *HostAlias {
	if in == nil {
		return nil
	}
	out := new(HostAlias)
	in.DeepCopyInto(out)
	return out
}

type PodDNSConfig struct {
	// A list of DNS name server IP addresses.
	// This will be appended to the base nameservers generated from DNSPolicy.
	// Duplicated nameservers will be removed.
	Nameservers []string `json:"nameservers"`
	// A list of DNS search domains for host-name lookup.
	// This will be appended to the base search paths generated from DNSPolicy.
	// Duplicated search paths will be removed.
	Searches []string `json:"searches"`
	// A list of DNS resolver options.
	// This will be merged with the base options generated from DNSPolicy.
	// Duplicated entries will be removed. Resolution options given in Options
	// will override those that appear in the base DNSPolicy.
	Options []PodDNSConfigOption `json:"options"`
}

func (in *PodDNSConfig) DeepCopyInto(out *PodDNSConfig) {
	*out = *in
	if in.Nameservers != nil {
		t := make([]string, len(in.Nameservers))
		copy(t, in.Nameservers)
		out.Nameservers = t
	}
	if in.Searches != nil {
		t := make([]string, len(in.Searches))
		copy(t, in.Searches)
		out.Searches = t
	}
	if in.Options != nil {
		l := make([]PodDNSConfigOption, len(in.Options))
		for i := range in.Options {
			in.Options[i].DeepCopyInto(&l[i])
		}
		out.Options = l
	}
}

func (in *PodDNSConfig) DeepCopy() *PodDNSConfig {
	if in == nil {
		return nil
	}
	out := new(PodDNSConfig)
	in.DeepCopyInto(out)
	return out
}

type PodReadinessGate struct {
	// ConditionType refers to a condition in the pod's condition list with matching type.
	ConditionType PodConditionType `json:"conditionType"`
}

func (in *PodReadinessGate) DeepCopyInto(out *PodReadinessGate) {
	*out = *in
}

func (in *PodReadinessGate) DeepCopy() *PodReadinessGate {
	if in == nil {
		return nil
	}
	out := new(PodReadinessGate)
	in.DeepCopyInto(out)
	return out
}

type TopologySpreadConstraint struct {
	// MaxSkew describes the degree to which pods may be unevenly distributed.
	// When `whenUnsatisfiable=DoNotSchedule`, it is the maximum permitted difference
	// between the number of matching pods in the target topology and the global minimum.
	// The global minimum is the minimum number of matching pods in an eligible domain
	// or zero if the number of eligible domains is less than MinDomains.
	// For example, in a 3-zone cluster, MaxSkew is set to 1, and pods with the same
	// labelSelector spread as 2/2/1:
	// In this case, the global minimum is 1.
	// | zone1 | zone2 | zone3 |
	// |  P P  |  P P  |   P   |
	// - if MaxSkew is 1, incoming pod can only be scheduled to zone3 to become 2/2/2;
	// scheduling it onto zone1(zone2) would make the ActualSkew(3-1) on zone1(zone2)
	// violate MaxSkew(1).
	// - if MaxSkew is 2, incoming pod can be scheduled onto any zone.
	// When `whenUnsatisfiable=ScheduleAnyway`, it is used to give higher precedence
	// to topologies that satisfy it.
	// It's a required field. Default value is 1 and 0 is not allowed.
	MaxSkew int `json:"maxSkew"`
	// TopologyKey is the key of node labels. Nodes that have a label with this key
	// and identical values are considered to be in the same topology.
	// We consider each <key, value> as a "bucket", and try to put balanced number
	// of pods into each bucket.
	// We define a domain as a particular instance of a topology.
	// Also, we define an eligible domain as a domain whose nodes meet the requirements of
	// nodeAffinityPolicy and nodeTaintsPolicy.
	// e.g. If TopologyKey is "kubernetes.io/hostname", each Node is a domain of that topology.
	// And, if TopologyKey is "topology.kubernetes.io/zone", each zone is a domain of that topology.
	// It's a required field.
	TopologyKey string `json:"topologyKey"`
	// WhenUnsatisfiable indicates how to deal with a pod if it doesn't satisfy
	// the spread constraint.
	// - DoNotSchedule (default) tells the scheduler not to schedule it.
	// - ScheduleAnyway tells the scheduler to schedule the pod in any location,
	// but giving higher precedence to topologies that would help reduce the
	// skew.
	// A constraint is considered "Unsatisfiable" for an incoming pod
	// if and only if every possible node assignment for that pod would violate
	// "MaxSkew" on some topology.
	// For example, in a 3-zone cluster, MaxSkew is set to 1, and pods with the same
	// labelSelector spread as 3/1/1:
	// | zone1 | zone2 | zone3 |
	// | P P P |   P   |   P   |
	// If WhenUnsatisfiable is set to DoNotSchedule, incoming pod can only be scheduled
	// to zone2(zone3) to become 3/2/1(3/1/2) as ActualSkew(2-1) on zone2(zone3) satisfies
	// MaxSkew(1). In other words, the cluster can still be imbalanced, but scheduler
	// won't make it *more* imbalanced.
	// It's a required field.
	WhenUnsatisfiable UnsatisfiableConstraintAction `json:"whenUnsatisfiable"`
	// LabelSelector is used to find matching pods.
	// Pods that match this label selector are counted to determine the number of pods
	// in their corresponding topology domain.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// MinDomains indicates a minimum number of eligible domains.
	// When the number of eligible domains with matching topology keys is less than minDomains,
	// Pod Topology Spread treats "global minimum" as 0, and then the calculation of Skew is performed.
	// And when the number of eligible domains with matching topology keys equals or greater than minDomains,
	// this value has no effect on scheduling.
	// As a result, when the number of eligible domains is less than minDomains,
	// scheduler won't schedule more than maxSkew Pods to those domains.
	// If value is nil, the constraint behaves as if MinDomains is equal to 1.
	// Valid values are integers greater than 0.
	// When value is not nil, WhenUnsatisfiable must be DoNotSchedule.
	// For example, in a 3-zone cluster, MaxSkew is set to 2, MinDomains is set to 5 and pods with the same
	// labelSelector spread as 2/2/2:
	// | zone1 | zone2 | zone3 |
	// |  P P  |  P P  |  P P  |
	// The number of domains is less than 5(MinDomains), so "global minimum" is treated as 0.
	// In this situation, new pod with the same labelSelector cannot be scheduled,
	// because computed skew will be 3(3 - 0) if new Pod is scheduled to any of the three zones,
	// it will violate MaxSkew.
	// This is a beta field and requires the MinDomainsInPodTopologySpread feature gate to be enabled (enabled by default).
	MinDomains int `json:"minDomains,omitempty"`
	// NodeAffinityPolicy indicates how we will treat Pod's nodeAffinity/nodeSelector
	// when calculating pod topology spread skew. Options are:
	// - Honor: only nodes matching nodeAffinity/nodeSelector are included in the calculations.
	// - Ignore: nodeAffinity/nodeSelector are ignored. All nodes are included in the calculations.
	// If this value is nil, the behavior is equivalent to the Honor policy.
	// This is a beta-level feature default enabled by the NodeInclusionPolicyInPodTopologySpread feature flag.
	NodeAffinityPolicy NodeInclusionPolicy `json:"nodeAffinityPolicy,omitempty"`
	// NodeTaintsPolicy indicates how we will treat node taints when calculating
	// pod topology spread skew. Options are:
	// - Honor: nodes without taints, along with tainted nodes for which the incoming pod
	// has a toleration, are included.
	// - Ignore: node taints are ignored. All nodes are included.
	// If this value is nil, the behavior is equivalent to the Ignore policy.
	// This is a beta-level feature default enabled by the NodeInclusionPolicyInPodTopologySpread feature flag.
	NodeTaintsPolicy NodeInclusionPolicy `json:"nodeTaintsPolicy,omitempty"`
	// MatchLabelKeys is a set of pod label keys to select the pods over which
	// spreading will be calculated. The keys are used to lookup values from the
	// incoming pod labels, those key-value labels are ANDed with labelSelector
	// to select the group of existing pods over which spreading will be calculated
	// for the incoming pod. The same key is forbidden to exist in both MatchLabelKeys and LabelSelector.
	// MatchLabelKeys cannot be set when LabelSelector isn't set.
	// Keys that don't exist in the incoming pod labels will
	// be ignored. A null or empty list means only match against labelSelector.
	// This is a beta field and requires the MatchLabelKeysInPodTopologySpread feature gate to be enabled (enabled by default).
	MatchLabelKeys []string `json:"matchLabelKeys"`
}

func (in *TopologySpreadConstraint) DeepCopyInto(out *TopologySpreadConstraint) {
	*out = *in
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.MatchLabelKeys != nil {
		t := make([]string, len(in.MatchLabelKeys))
		copy(t, in.MatchLabelKeys)
		out.MatchLabelKeys = t
	}
}

func (in *TopologySpreadConstraint) DeepCopy() *TopologySpreadConstraint {
	if in == nil {
		return nil
	}
	out := new(TopologySpreadConstraint)
	in.DeepCopyInto(out)
	return out
}

type PodOS struct {
	// Name is the name of the operating system. The currently supported values are linux and windows.
	// Additional value may be defined in future and can be one of:
	// https://github.com/opencontainers/runtime-spec/blob/master/config.md#platform-specific-configuration
	// Clients should expect to handle additional values and treat unrecognized values in this field as os: null
	Name OSName `json:"name"`
}

func (in *PodOS) DeepCopyInto(out *PodOS) {
	*out = *in
}

func (in *PodOS) DeepCopy() *PodOS {
	if in == nil {
		return nil
	}
	out := new(PodOS)
	in.DeepCopyInto(out)
	return out
}

type PodSchedulingGate struct {
	// Name of the scheduling gate.
	// Each scheduling gate must have a unique name field.
	Name string `json:"name"`
}

func (in *PodSchedulingGate) DeepCopyInto(out *PodSchedulingGate) {
	*out = *in
}

func (in *PodSchedulingGate) DeepCopy() *PodSchedulingGate {
	if in == nil {
		return nil
	}
	out := new(PodSchedulingGate)
	in.DeepCopyInto(out)
	return out
}

type PodResourceClaim struct {
	// Name uniquely identifies this resource claim inside the pod.
	// This must be a DNS_LABEL.
	Name string `json:"name"`
	// Source describes where to find the ResourceClaim.
	Source *ClaimSource `json:"source,omitempty"`
}

func (in *PodResourceClaim) DeepCopyInto(out *PodResourceClaim) {
	*out = *in
	if in.Source != nil {
		in, out := &in.Source, &out.Source
		*out = new(ClaimSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodResourceClaim) DeepCopy() *PodResourceClaim {
	if in == nil {
		return nil
	}
	out := new(PodResourceClaim)
	in.DeepCopyInto(out)
	return out
}

type PodCondition struct {
	// Type is the type of the condition.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Type PodConditionType `json:"type"`
	// Status is the status of the condition.
	// Can be True, False, Unknown.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#pod-conditions
	Status ConditionStatus `json:"status"`
	// Last time we probed the condition.
	LastProbeTime *metav1.Time `json:"lastProbeTime,omitempty"`
	// Last time the condition transitioned from one status to another.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// Unique, one-word, CamelCase reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// Human-readable message indicating details about last transition.
	Message string `json:"message,omitempty"`
}

func (in *PodCondition) DeepCopyInto(out *PodCondition) {
	*out = *in
	if in.LastProbeTime != nil {
		in, out := &in.LastProbeTime, &out.LastProbeTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodCondition) DeepCopy() *PodCondition {
	if in == nil {
		return nil
	}
	out := new(PodCondition)
	in.DeepCopyInto(out)
	return out
}

type PodIP struct {
	// ip is an IP address (IPv4 or IPv6) assigned to the pod
	IP string `json:"ip,omitempty"`
}

func (in *PodIP) DeepCopyInto(out *PodIP) {
	*out = *in
}

func (in *PodIP) DeepCopy() *PodIP {
	if in == nil {
		return nil
	}
	out := new(PodIP)
	in.DeepCopyInto(out)
	return out
}

type ContainerStatus struct {
	// Name is a DNS_LABEL representing the unique name of the container.
	// Each container in a pod must have a unique name across all container types.
	// Cannot be updated.
	Name string `json:"name"`
	// State holds details about the container's current condition.
	State *ContainerState `json:"state,omitempty"`
	// LastTerminationState holds the last termination state of the container to
	// help debug container crashes and restarts. This field is not
	// populated if the container is still running and RestartCount is 0.
	LastTerminationState *ContainerState `json:"lastState,omitempty"`
	// Ready specifies whether the container is currently passing its readiness check.
	// The value will change as readiness probes keep executing. If no readiness
	// probes are specified, this field defaults to true once the container is
	// fully started (see Started field).
	// The value is typically used to determine whether a container is ready to
	// accept traffic.
	Ready bool `json:"ready"`
	// RestartCount holds the number of times the container has been restarted.
	// Kubelet makes an effort to always increment the value, but there
	// are cases when the state may be lost due to node restarts and then the value
	// may be reset to 0. The value is never negative.
	RestartCount int `json:"restartCount"`
	// Image is the name of container image that the container is running.
	// The container image may not match the image used in the PodSpec,
	// as it may have been resolved by the runtime.
	// More info: https://kubernetes.io/docs/concepts/containers/images.
	Image string `json:"image"`
	// ImageID is the image ID of the container's image. The image ID may not
	// match the image ID of the image used in the PodSpec, as it may have been
	// resolved by the runtime.
	ImageID string `json:"imageID"`
	// ContainerID is the ID of the container in the format '<type>://<container_id>'.
	// Where type is a container runtime identifier, returned from Version call of CRI API
	// (for example "containerd").
	ContainerID string `json:"containerID,omitempty"`
	// Started indicates whether the container has finished its postStart lifecycle hook
	// and passed its startup probe.
	// Initialized as false, becomes true after startupProbe is considered
	// successful. Resets to false when the container is restarted, or if kubelet
	// loses state temporarily. In both cases, startup probes will run again.
	// Is always true when no startupProbe is defined and container is running and
	// has passed the postStart lifecycle hook. The null value must be treated the
	// same as false.
	Started bool `json:"started,omitempty"`
	// AllocatedResources represents the compute resources allocated for this container by the
	// node. Kubelet sets this value to Container.Resources.Requests upon successful pod admission
	// and after successfully admitting desired pod resize.
	AllocatedResources map[string]apiresource.Quantity `json:"allocatedResources,omitempty"`
	// Resources represents the compute resource requests and limits that have been successfully
	// enacted on the running container after it has been started or has been successfully resized.
	Resources *ResourceRequirements `json:"resources,omitempty"`
}

func (in *ContainerStatus) DeepCopyInto(out *ContainerStatus) {
	*out = *in
	if in.State != nil {
		in, out := &in.State, &out.State
		*out = new(ContainerState)
		(*in).DeepCopyInto(*out)
	}
	if in.LastTerminationState != nil {
		in, out := &in.LastTerminationState, &out.LastTerminationState
		*out = new(ContainerState)
		(*in).DeepCopyInto(*out)
	}
	if in.AllocatedResources != nil {
		in, out := &in.AllocatedResources, &out.AllocatedResources
		*out = make(map[string]apiresource.Quantity, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ContainerStatus) DeepCopy() *ContainerStatus {
	if in == nil {
		return nil
	}
	out := new(ContainerStatus)
	in.DeepCopyInto(out)
	return out
}

type ReplicationControllerCondition struct {
	// Type of replication controller condition.
	Type ReplicationControllerConditionType `json:"type"`
	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// The last time the condition transitioned from one status to another.
	LastTransitionTime *metav1.Time `json:"lastTransitionTime,omitempty"`
	// The reason for the condition's last transition.
	Reason string `json:"reason,omitempty"`
	// A human readable message indicating details about the transition.
	Message string `json:"message,omitempty"`
}

func (in *ReplicationControllerCondition) DeepCopyInto(out *ReplicationControllerCondition) {
	*out = *in
	if in.LastTransitionTime != nil {
		in, out := &in.LastTransitionTime, &out.LastTransitionTime
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ReplicationControllerCondition) DeepCopy() *ReplicationControllerCondition {
	if in == nil {
		return nil
	}
	out := new(ReplicationControllerCondition)
	in.DeepCopyInto(out)
	return out
}

type ScopeSelector struct {
	// A list of scope selector requirements by scope of the resources.
	MatchExpressions []ScopedResourceSelectorRequirement `json:"matchExpressions"`
}

func (in *ScopeSelector) DeepCopyInto(out *ScopeSelector) {
	*out = *in
	if in.MatchExpressions != nil {
		l := make([]ScopedResourceSelectorRequirement, len(in.MatchExpressions))
		for i := range in.MatchExpressions {
			in.MatchExpressions[i].DeepCopyInto(&l[i])
		}
		out.MatchExpressions = l
	}
}

func (in *ScopeSelector) DeepCopy() *ScopeSelector {
	if in == nil {
		return nil
	}
	out := new(ScopeSelector)
	in.DeepCopyInto(out)
	return out
}

type ServicePort struct {
	// The name of this port within the service. This must be a DNS_LABEL.
	// All ports within a ServiceSpec must have unique names. When considering
	// the endpoints for a Service, this must match the 'name' field in the
	// EndpointPort.
	// Optional if only one ServicePort is defined on this service.
	Name string `json:"name,omitempty"`
	// The IP protocol for this port. Supports "TCP", "UDP", and "SCTP".
	// Default is TCP.
	Protocol Protocol `json:"protocol,omitempty"`
	// The application protocol for this port.
	// This field follows standard Kubernetes label syntax.
	// Un-prefixed names are reserved for IANA standard service names (as per
	// RFC-6335 and https://www.iana.org/assignments/service-names).
	// Non-standard protocols should use prefixed names such as
	// mycompany.com/my-custom-protocol.
	AppProtocol string `json:"appProtocol,omitempty"`
	// The port that will be exposed by this service.
	Port int `json:"port"`
	// Number or name of the port to access on the pods targeted by the service.
	// Number must be in the range 1 to 65535. Name must be an IANA_SVC_NAME.
	// If this is a string, it will be looked up as a named port in the
	// target Pod's container ports. If this is not specified, the value
	// of the 'port' field is used (an identity map).
	// This field is ignored for services with clusterIP=None, and should be
	// omitted or set equal to the 'port' field.
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#defining-a-service
	TargetPort *utilintstr.IntOrString `json:"targetPort,omitempty"`
	// The port on each node on which this service is exposed when type is
	// NodePort or LoadBalancer.  Usually assigned by the system. If a value is
	// specified, in-range, and not in use it will be used, otherwise the
	// operation will fail.  If not specified, a port will be allocated if this
	// Service requires one.  If this field is specified when creating a
	// Service which does not need it, creation will fail. This field will be
	// wiped when updating a Service to no longer need it (e.g. changing type
	// from NodePort to ClusterIP).
	// More info: https://kubernetes.io/docs/concepts/services-networking/service/#type-nodeport
	NodePort int `json:"nodePort,omitempty"`
}

func (in *ServicePort) DeepCopyInto(out *ServicePort) {
	*out = *in
	if in.TargetPort != nil {
		in, out := &in.TargetPort, &out.TargetPort
		*out = new(utilintstr.IntOrString)
		*out = *in
	}
}

func (in *ServicePort) DeepCopy() *ServicePort {
	if in == nil {
		return nil
	}
	out := new(ServicePort)
	in.DeepCopyInto(out)
	return out
}

type SessionAffinityConfig struct {
	// clientIP contains the configurations of Client IP based session affinity.
	ClientIP *ClientIPConfig `json:"clientIP,omitempty"`
}

func (in *SessionAffinityConfig) DeepCopyInto(out *SessionAffinityConfig) {
	*out = *in
	if in.ClientIP != nil {
		in, out := &in.ClientIP, &out.ClientIP
		*out = new(ClientIPConfig)
		(*in).DeepCopyInto(*out)
	}
}

func (in *SessionAffinityConfig) DeepCopy() *SessionAffinityConfig {
	if in == nil {
		return nil
	}
	out := new(SessionAffinityConfig)
	in.DeepCopyInto(out)
	return out
}

type LoadBalancerStatus struct {
	// Ingress is a list containing ingress points for the load-balancer.
	// Traffic intended for the service should be sent to these ingress points.
	Ingress []LoadBalancerIngress `json:"ingress"`
}

func (in *LoadBalancerStatus) DeepCopyInto(out *LoadBalancerStatus) {
	*out = *in
	if in.Ingress != nil {
		l := make([]LoadBalancerIngress, len(in.Ingress))
		for i := range in.Ingress {
			in.Ingress[i].DeepCopyInto(&l[i])
		}
		out.Ingress = l
	}
}

func (in *LoadBalancerStatus) DeepCopy() *LoadBalancerStatus {
	if in == nil {
		return nil
	}
	out := new(LoadBalancerStatus)
	in.DeepCopyInto(out)
	return out
}

type ConfigMapNodeConfigSource struct {
	// Namespace is the metadata.namespace of the referenced ConfigMap.
	// This field is required in all cases.
	Namespace string `json:"namespace"`
	// Name is the metadata.name of the referenced ConfigMap.
	// This field is required in all cases.
	Name string `json:"name"`
	// UID is the metadata.UID of the referenced ConfigMap.
	// This field is forbidden in Node.Spec, and required in Node.Status.
	UID string `json:"uid,omitempty"`
	// ResourceVersion is the metadata.ResourceVersion of the referenced ConfigMap.
	// This field is forbidden in Node.Spec, and required in Node.Status.
	ResourceVersion string `json:"resourceVersion,omitempty"`
	// KubeletConfigKey declares which key of the referenced ConfigMap corresponds to the KubeletConfiguration structure
	// This field is required in all cases.
	KubeletConfigKey string `json:"kubeletConfigKey"`
}

func (in *ConfigMapNodeConfigSource) DeepCopyInto(out *ConfigMapNodeConfigSource) {
	*out = *in
}

func (in *ConfigMapNodeConfigSource) DeepCopy() *ConfigMapNodeConfigSource {
	if in == nil {
		return nil
	}
	out := new(ConfigMapNodeConfigSource)
	in.DeepCopyInto(out)
	return out
}

type DaemonEndpoint struct {
	// Port number of the given endpoint.
	Port int `json:"Port"`
}

func (in *DaemonEndpoint) DeepCopyInto(out *DaemonEndpoint) {
	*out = *in
}

func (in *DaemonEndpoint) DeepCopy() *DaemonEndpoint {
	if in == nil {
		return nil
	}
	out := new(DaemonEndpoint)
	in.DeepCopyInto(out)
	return out
}

type GCEPersistentDiskVolumeSource struct {
	// pdName is unique name of the PD resource in GCE. Used to identify the disk in GCE.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
	PDName string `json:"pdName"`
	// fsType is filesystem type of the volume that you want to mount.
	// Tip: Ensure that the filesystem type is supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
	FSType string `json:"fsType,omitempty"`
	// partition is the partition in the volume that you want to mount.
	// If omitted, the default is to mount by volume name.
	// Examples: For volume /dev/sda1, you specify the partition as "1".
	// Similarly, the volume partition for /dev/sda is "0" (or you can leave the property empty).
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
	Partition int `json:"partition,omitempty"`
	// readOnly here will force the ReadOnly setting in VolumeMounts.
	// Defaults to false.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *GCEPersistentDiskVolumeSource) DeepCopyInto(out *GCEPersistentDiskVolumeSource) {
	*out = *in
}

func (in *GCEPersistentDiskVolumeSource) DeepCopy() *GCEPersistentDiskVolumeSource {
	if in == nil {
		return nil
	}
	out := new(GCEPersistentDiskVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type AWSElasticBlockStoreVolumeSource struct {
	// volumeID is unique ID of the persistent disk resource in AWS (Amazon EBS volume).
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
	VolumeID string `json:"volumeID"`
	// fsType is the filesystem type of the volume that you want to mount.
	// Tip: Ensure that the filesystem type is supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
	FSType string `json:"fsType,omitempty"`
	// partition is the partition in the volume that you want to mount.
	// If omitted, the default is to mount by volume name.
	// Examples: For volume /dev/sda1, you specify the partition as "1".
	// Similarly, the volume partition for /dev/sda is "0" (or you can leave the property empty).
	Partition int `json:"partition,omitempty"`
	// readOnly value true will force the readOnly setting in VolumeMounts.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *AWSElasticBlockStoreVolumeSource) DeepCopyInto(out *AWSElasticBlockStoreVolumeSource) {
	*out = *in
}

func (in *AWSElasticBlockStoreVolumeSource) DeepCopy() *AWSElasticBlockStoreVolumeSource {
	if in == nil {
		return nil
	}
	out := new(AWSElasticBlockStoreVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type HostPathVolumeSource struct {
	// path of the directory on the host.
	// If the path is a symlink, it will follow the link to the real path.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	Path string `json:"path"`
	// type for HostPath Volume
	// Defaults to ""
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	Type HostPathType `json:"type,omitempty"`
}

func (in *HostPathVolumeSource) DeepCopyInto(out *HostPathVolumeSource) {
	*out = *in
}

func (in *HostPathVolumeSource) DeepCopy() *HostPathVolumeSource {
	if in == nil {
		return nil
	}
	out := new(HostPathVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type GlusterfsPersistentVolumeSource struct {
	// endpoints is the endpoint name that details Glusterfs topology.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	EndpointsName string `json:"endpoints"`
	// path is the Glusterfs volume path.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	Path string `json:"path"`
	// readOnly here will force the Glusterfs volume to be mounted with read-only permissions.
	// Defaults to false.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	ReadOnly bool `json:"readOnly,omitempty"`
	// endpointsNamespace is the namespace that contains Glusterfs endpoint.
	// If this field is empty, the EndpointNamespace defaults to the same namespace as the bound PVC.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	EndpointsNamespace string `json:"endpointsNamespace,omitempty"`
}

func (in *GlusterfsPersistentVolumeSource) DeepCopyInto(out *GlusterfsPersistentVolumeSource) {
	*out = *in
}

func (in *GlusterfsPersistentVolumeSource) DeepCopy() *GlusterfsPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(GlusterfsPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type NFSVolumeSource struct {
	// server is the hostname or IP address of the NFS server.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
	Server string `json:"server"`
	// path that is exported by the NFS server.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
	Path string `json:"path"`
	// readOnly here will force the NFS export to be mounted with read-only permissions.
	// Defaults to false.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *NFSVolumeSource) DeepCopyInto(out *NFSVolumeSource) {
	*out = *in
}

func (in *NFSVolumeSource) DeepCopy() *NFSVolumeSource {
	if in == nil {
		return nil
	}
	out := new(NFSVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type RBDPersistentVolumeSource struct {
	// monitors is a collection of Ceph monitors.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	CephMonitors []string `json:"monitors"`
	// image is the rados image name.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	RBDImage string `json:"image"`
	// fsType is the filesystem type of the volume that you want to mount.
	// Tip: Ensure that the filesystem type is supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#rbd
	FSType string `json:"fsType,omitempty"`
	// pool is the rados pool name.
	// Default is rbd.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	RBDPool string `json:"pool,omitempty"`
	// user is the rados user name.
	// Default is admin.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	RadosUser string `json:"user,omitempty"`
	// keyring is the path to key ring for RBDUser.
	// Default is /etc/ceph/keyring.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	Keyring string `json:"keyring,omitempty"`
	// secretRef is name of the authentication secret for RBDUser. If provided
	// overrides keyring.
	// Default is nil.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	// readOnly here will force the ReadOnly setting in VolumeMounts.
	// Defaults to false.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *RBDPersistentVolumeSource) DeepCopyInto(out *RBDPersistentVolumeSource) {
	*out = *in
	if in.CephMonitors != nil {
		t := make([]string, len(in.CephMonitors))
		copy(t, in.CephMonitors)
		out.CephMonitors = t
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *RBDPersistentVolumeSource) DeepCopy() *RBDPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(RBDPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ISCSIPersistentVolumeSource struct {
	// targetPortal is iSCSI Target Portal. The Portal is either an IP or ip_addr:port if the port
	// is other than default (typically TCP ports 860 and 3260).
	TargetPortal string `json:"targetPortal"`
	// iqn is Target iSCSI Qualified Name.
	IQN string `json:"iqn"`
	// lun is iSCSI Target Lun number.
	Lun int `json:"lun"`
	// iscsiInterface is the interface Name that uses an iSCSI transport.
	// Defaults to 'default' (tcp).
	ISCSIInterface string `json:"iscsiInterface,omitempty"`
	// fsType is the filesystem type of the volume that you want to mount.
	// Tip: Ensure that the filesystem type is supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#iscsi
	FSType string `json:"fsType,omitempty"`
	// readOnly here will force the ReadOnly setting in VolumeMounts.
	// Defaults to false.
	ReadOnly bool `json:"readOnly,omitempty"`
	// portals is the iSCSI Target Portal List. The Portal is either an IP or ip_addr:port if the port
	// is other than default (typically TCP ports 860 and 3260).
	Portals []string `json:"portals"`
	// chapAuthDiscovery defines whether support iSCSI Discovery CHAP authentication
	DiscoveryCHAPAuth bool `json:"chapAuthDiscovery,omitempty"`
	// chapAuthSession defines whether support iSCSI Session CHAP authentication
	SessionCHAPAuth bool `json:"chapAuthSession,omitempty"`
	// secretRef is the CHAP Secret for iSCSI target and initiator authentication
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	// initiatorName is the custom iSCSI Initiator Name.
	// If initiatorName is specified with iscsiInterface simultaneously, new iSCSI interface
	// <target portal>:<volume name> will be created for the connection.
	InitiatorName string `json:"initiatorName,omitempty"`
}

func (in *ISCSIPersistentVolumeSource) DeepCopyInto(out *ISCSIPersistentVolumeSource) {
	*out = *in
	if in.Portals != nil {
		t := make([]string, len(in.Portals))
		copy(t, in.Portals)
		out.Portals = t
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ISCSIPersistentVolumeSource) DeepCopy() *ISCSIPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(ISCSIPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type CinderPersistentVolumeSource struct {
	// volumeID used to identify the volume in cinder.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	VolumeID string `json:"volumeID"`
	// fsType Filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	FSType string `json:"fsType,omitempty"`
	// readOnly is Optional: Defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	ReadOnly bool `json:"readOnly,omitempty"`
	// secretRef is Optional: points to a secret object containing parameters used to connect
	// to OpenStack.
	SecretRef *SecretReference `json:"secretRef,omitempty"`
}

func (in *CinderPersistentVolumeSource) DeepCopyInto(out *CinderPersistentVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *CinderPersistentVolumeSource) DeepCopy() *CinderPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(CinderPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type CephFSPersistentVolumeSource struct {
	// monitors is Required: Monitors is a collection of Ceph monitors
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	Monitors []string `json:"monitors"`
	// path is Optional: Used as the mounted root, rather than the full Ceph tree, default is /
	Path string `json:"path,omitempty"`
	// user is Optional: User is the rados user name, default is admin
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	User string `json:"user,omitempty"`
	// secretFile is Optional: SecretFile is the path to key ring for User, default is /etc/ceph/user.secret
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	SecretFile string `json:"secretFile,omitempty"`
	// secretRef is Optional: SecretRef is reference to the authentication secret for User, default is empty.
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	// readOnly is Optional: Defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *CephFSPersistentVolumeSource) DeepCopyInto(out *CephFSPersistentVolumeSource) {
	*out = *in
	if in.Monitors != nil {
		t := make([]string, len(in.Monitors))
		copy(t, in.Monitors)
		out.Monitors = t
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *CephFSPersistentVolumeSource) DeepCopy() *CephFSPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(CephFSPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type FCVolumeSource struct {
	// targetWWNs is Optional: FC target worldwide names (WWNs)
	TargetWWNs []string `json:"targetWWNs"`
	// lun is Optional: FC target lun number
	Lun int `json:"lun,omitempty"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
	// readOnly is Optional: Defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// wwids Optional: FC volume world wide identifiers (wwids)
	// Either wwids or combination of targetWWNs and lun must be set, but not both simultaneously.
	WWIDs []string `json:"wwids"`
}

func (in *FCVolumeSource) DeepCopyInto(out *FCVolumeSource) {
	*out = *in
	if in.TargetWWNs != nil {
		t := make([]string, len(in.TargetWWNs))
		copy(t, in.TargetWWNs)
		out.TargetWWNs = t
	}
	if in.WWIDs != nil {
		t := make([]string, len(in.WWIDs))
		copy(t, in.WWIDs)
		out.WWIDs = t
	}
}

func (in *FCVolumeSource) DeepCopy() *FCVolumeSource {
	if in == nil {
		return nil
	}
	out := new(FCVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type FlockerVolumeSource struct {
	// datasetName is Name of the dataset stored as metadata -> name on the dataset for Flocker
	// should be considered as deprecated
	DatasetName string `json:"datasetName,omitempty"`
	// datasetUUID is the UUID of the dataset. This is unique identifier of a Flocker dataset
	DatasetUUID string `json:"datasetUUID,omitempty"`
}

func (in *FlockerVolumeSource) DeepCopyInto(out *FlockerVolumeSource) {
	*out = *in
}

func (in *FlockerVolumeSource) DeepCopy() *FlockerVolumeSource {
	if in == nil {
		return nil
	}
	out := new(FlockerVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type FlexPersistentVolumeSource struct {
	// driver is the name of the driver to use for this volume.
	Driver string `json:"driver"`
	// fsType is the Filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". The default filesystem depends on FlexVolume script.
	FSType string `json:"fsType,omitempty"`
	// secretRef is Optional: SecretRef is reference to the secret object containing
	// sensitive information to pass to the plugin scripts. This may be
	// empty if no secret object is specified. If the secret object
	// contains more than one secret, all secrets are passed to the plugin
	// scripts.
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	// readOnly is Optional: defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// options is Optional: this field holds extra command options if any.
	Options map[string]string `json:"options,omitempty"`
}

func (in *FlexPersistentVolumeSource) DeepCopyInto(out *FlexPersistentVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *FlexPersistentVolumeSource) DeepCopy() *FlexPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(FlexPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type AzureFilePersistentVolumeSource struct {
	// secretName is the name of secret that contains Azure Storage Account Name and Key
	SecretName string `json:"secretName"`
	// shareName is the azure Share Name
	ShareName string `json:"shareName"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// secretNamespace is the namespace of the secret that contains Azure Storage Account Name and Key
	// default is the same as the Pod
	SecretNamespace string `json:"secretNamespace,omitempty"`
}

func (in *AzureFilePersistentVolumeSource) DeepCopyInto(out *AzureFilePersistentVolumeSource) {
	*out = *in
}

func (in *AzureFilePersistentVolumeSource) DeepCopy() *AzureFilePersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(AzureFilePersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type VsphereVirtualDiskVolumeSource struct {
	// volumePath is the path that identifies vSphere volume vmdk
	VolumePath string `json:"volumePath"`
	// fsType is filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
	// storagePolicyName is the storage Policy Based Management (SPBM) profile name.
	StoragePolicyName string `json:"storagePolicyName,omitempty"`
	// storagePolicyID is the storage Policy Based Management (SPBM) profile ID associated with the StoragePolicyName.
	StoragePolicyID string `json:"storagePolicyID,omitempty"`
}

func (in *VsphereVirtualDiskVolumeSource) DeepCopyInto(out *VsphereVirtualDiskVolumeSource) {
	*out = *in
}

func (in *VsphereVirtualDiskVolumeSource) DeepCopy() *VsphereVirtualDiskVolumeSource {
	if in == nil {
		return nil
	}
	out := new(VsphereVirtualDiskVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type QuobyteVolumeSource struct {
	// registry represents a single or multiple Quobyte Registry services
	// specified as a string as host:port pair (multiple entries are separated with commas)
	// which acts as the central registry for volumes
	Registry string `json:"registry"`
	// volume is a string that references an already created Quobyte volume by name.
	Volume string `json:"volume"`
	// readOnly here will force the Quobyte volume to be mounted with read-only permissions.
	// Defaults to false.
	ReadOnly bool `json:"readOnly,omitempty"`
	// user to map volume access to
	// Defaults to serivceaccount user
	User string `json:"user,omitempty"`
	// group to map volume access to
	// Default is no group
	Group string `json:"group,omitempty"`
	// tenant owning the given Quobyte volume in the Backend
	// Used with dynamically provisioned Quobyte volumes, value is set by the plugin
	Tenant string `json:"tenant,omitempty"`
}

func (in *QuobyteVolumeSource) DeepCopyInto(out *QuobyteVolumeSource) {
	*out = *in
}

func (in *QuobyteVolumeSource) DeepCopy() *QuobyteVolumeSource {
	if in == nil {
		return nil
	}
	out := new(QuobyteVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type AzureDiskVolumeSource struct {
	// diskName is the Name of the data disk in the blob storage
	DiskName string `json:"diskName"`
	// diskURI is the URI of data disk in the blob storage
	DataDiskURI string `json:"diskURI"`
	// cachingMode is the Host Caching mode: None, Read Only, Read Write.
	CachingMode AzureDataDiskCachingMode `json:"cachingMode,omitempty"`
	// fsType is Filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
	// readOnly Defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// kind expected values are Shared: multiple blob disks per storage account  Dedicated: single blob disk per storage account  Managed: azure managed data disk (only in managed availability set). defaults to shared
	Kind AzureDataDiskKind `json:"kind,omitempty"`
}

func (in *AzureDiskVolumeSource) DeepCopyInto(out *AzureDiskVolumeSource) {
	*out = *in
}

func (in *AzureDiskVolumeSource) DeepCopy() *AzureDiskVolumeSource {
	if in == nil {
		return nil
	}
	out := new(AzureDiskVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type PhotonPersistentDiskVolumeSource struct {
	// pdID is the ID that identifies Photon Controller persistent disk
	PdID string `json:"pdID"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
}

func (in *PhotonPersistentDiskVolumeSource) DeepCopyInto(out *PhotonPersistentDiskVolumeSource) {
	*out = *in
}

func (in *PhotonPersistentDiskVolumeSource) DeepCopy() *PhotonPersistentDiskVolumeSource {
	if in == nil {
		return nil
	}
	out := new(PhotonPersistentDiskVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type PortworxVolumeSource struct {
	// volumeID uniquely identifies a Portworx volume
	VolumeID string `json:"volumeID"`
	// fSType represents the filesystem type to mount
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *PortworxVolumeSource) DeepCopyInto(out *PortworxVolumeSource) {
	*out = *in
}

func (in *PortworxVolumeSource) DeepCopy() *PortworxVolumeSource {
	if in == nil {
		return nil
	}
	out := new(PortworxVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ScaleIOPersistentVolumeSource struct {
	// gateway is the host address of the ScaleIO API Gateway.
	Gateway string `json:"gateway"`
	// system is the name of the storage system as configured in ScaleIO.
	System string `json:"system"`
	// secretRef references to the secret for ScaleIO user and other
	// sensitive information. If this is not provided, Login operation will fail.
	SecretRef *SecretReference `json:"secretRef,omitempty"`
	// sslEnabled is the flag to enable/disable SSL communication with Gateway, default false
	SSLEnabled bool `json:"sslEnabled,omitempty"`
	// protectionDomain is the name of the ScaleIO Protection Domain for the configured storage.
	ProtectionDomain string `json:"protectionDomain,omitempty"`
	// storagePool is the ScaleIO Storage Pool associated with the protection domain.
	StoragePool string `json:"storagePool,omitempty"`
	// storageMode indicates whether the storage for a volume should be ThickProvisioned or ThinProvisioned.
	// Default is ThinProvisioned.
	StorageMode string `json:"storageMode,omitempty"`
	// volumeName is the name of a volume already created in the ScaleIO system
	// that is associated with this volume source.
	VolumeName string `json:"volumeName,omitempty"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs".
	// Default is "xfs"
	FSType string `json:"fsType,omitempty"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *ScaleIOPersistentVolumeSource) DeepCopyInto(out *ScaleIOPersistentVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ScaleIOPersistentVolumeSource) DeepCopy() *ScaleIOPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(ScaleIOPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type LocalVolumeSource struct {
	// path of the full path to the volume on the node.
	// It can be either a directory or block device (disk, partition, ...).
	Path string `json:"path"`
	// fsType is the filesystem type to mount.
	// It applies only when the Path is a block device.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". The default value is to auto-select a filesystem if unspecified.
	FSType string `json:"fsType,omitempty"`
}

func (in *LocalVolumeSource) DeepCopyInto(out *LocalVolumeSource) {
	*out = *in
}

func (in *LocalVolumeSource) DeepCopy() *LocalVolumeSource {
	if in == nil {
		return nil
	}
	out := new(LocalVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type StorageOSPersistentVolumeSource struct {
	// volumeName is the human-readable name of the StorageOS volume.  Volume
	// names are only unique within a namespace.
	VolumeName string `json:"volumeName,omitempty"`
	// volumeNamespace specifies the scope of the volume within StorageOS.  If no
	// namespace is specified then the Pod's namespace will be used.  This allows the
	// Kubernetes name scoping to be mirrored within StorageOS for tighter integration.
	// Set VolumeName to any name to override the default behaviour.
	// Set to "default" if you are not using namespaces within StorageOS.
	// Namespaces that do not pre-exist within StorageOS will be created.
	VolumeNamespace string `json:"volumeNamespace,omitempty"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// secretRef specifies the secret to use for obtaining the StorageOS API
	// credentials.  If not specified, default values will be attempted.
	SecretRef *ObjectReference `json:"secretRef,omitempty"`
}

func (in *StorageOSPersistentVolumeSource) DeepCopyInto(out *StorageOSPersistentVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(ObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *StorageOSPersistentVolumeSource) DeepCopy() *StorageOSPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(StorageOSPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type CSIPersistentVolumeSource struct {
	// driver is the name of the driver to use for this volume.
	// Required.
	Driver string `json:"driver"`
	// volumeHandle is the unique volume name returned by the CSI volume
	// plugin’s CreateVolume to refer to the volume on all subsequent calls.
	// Required.
	VolumeHandle string `json:"volumeHandle"`
	// readOnly value to pass to ControllerPublishVolumeRequest.
	// Defaults to false (read/write).
	ReadOnly bool `json:"readOnly,omitempty"`
	// fsType to mount. Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs".
	FSType string `json:"fsType,omitempty"`
	// volumeAttributes of the volume to publish.
	VolumeAttributes map[string]string `json:"volumeAttributes,omitempty"`
	// controllerPublishSecretRef is a reference to the secret object containing
	// sensitive information to pass to the CSI driver to complete the CSI
	// ControllerPublishVolume and ControllerUnpublishVolume calls.
	// This field is optional, and may be empty if no secret is required. If the
	// secret object contains more than one secret, all secrets are passed.
	ControllerPublishSecretRef *SecretReference `json:"controllerPublishSecretRef,omitempty"`
	// nodeStageSecretRef is a reference to the secret object containing sensitive
	// information to pass to the CSI driver to complete the CSI NodeStageVolume
	// and NodeStageVolume and NodeUnstageVolume calls.
	// This field is optional, and may be empty if no secret is required. If the
	// secret object contains more than one secret, all secrets are passed.
	NodeStageSecretRef *SecretReference `json:"nodeStageSecretRef,omitempty"`
	// nodePublishSecretRef is a reference to the secret object containing
	// sensitive information to pass to the CSI driver to complete the CSI
	// NodePublishVolume and NodeUnpublishVolume calls.
	// This field is optional, and may be empty if no secret is required. If the
	// secret object contains more than one secret, all secrets are passed.
	NodePublishSecretRef *SecretReference `json:"nodePublishSecretRef,omitempty"`
	// controllerExpandSecretRef is a reference to the secret object containing
	// sensitive information to pass to the CSI driver to complete the CSI
	// ControllerExpandVolume call.
	// This field is optional, and may be empty if no secret is required. If the
	// secret object contains more than one secret, all secrets are passed.
	ControllerExpandSecretRef *SecretReference `json:"controllerExpandSecretRef,omitempty"`
	// nodeExpandSecretRef is a reference to the secret object containing
	// sensitive information to pass to the CSI driver to complete the CSI
	// NodeExpandVolume call.
	// This is a beta field which is enabled default by CSINodeExpandSecret feature gate.
	// This field is optional, may be omitted if no secret is required. If the
	// secret object contains more than one secret, all secrets are passed.
	NodeExpandSecretRef *SecretReference `json:"nodeExpandSecretRef,omitempty"`
}

func (in *CSIPersistentVolumeSource) DeepCopyInto(out *CSIPersistentVolumeSource) {
	*out = *in
	if in.VolumeAttributes != nil {
		in, out := &in.VolumeAttributes, &out.VolumeAttributes
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.ControllerPublishSecretRef != nil {
		in, out := &in.ControllerPublishSecretRef, &out.ControllerPublishSecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeStageSecretRef != nil {
		in, out := &in.NodeStageSecretRef, &out.NodeStageSecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
	if in.NodePublishSecretRef != nil {
		in, out := &in.NodePublishSecretRef, &out.NodePublishSecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
	if in.ControllerExpandSecretRef != nil {
		in, out := &in.ControllerExpandSecretRef, &out.ControllerExpandSecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
	if in.NodeExpandSecretRef != nil {
		in, out := &in.NodeExpandSecretRef, &out.NodeExpandSecretRef
		*out = new(SecretReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *CSIPersistentVolumeSource) DeepCopy() *CSIPersistentVolumeSource {
	if in == nil {
		return nil
	}
	out := new(CSIPersistentVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type NodeSelector struct {
	// Required. A list of node selector terms. The terms are ORed.
	NodeSelectorTerms []NodeSelectorTerm `json:"nodeSelectorTerms"`
}

func (in *NodeSelector) DeepCopyInto(out *NodeSelector) {
	*out = *in
	if in.NodeSelectorTerms != nil {
		l := make([]NodeSelectorTerm, len(in.NodeSelectorTerms))
		for i := range in.NodeSelectorTerms {
			in.NodeSelectorTerms[i].DeepCopyInto(&l[i])
		}
		out.NodeSelectorTerms = l
	}
}

func (in *NodeSelector) DeepCopy() *NodeSelector {
	if in == nil {
		return nil
	}
	out := new(NodeSelector)
	in.DeepCopyInto(out)
	return out
}

type ResourceClaim struct {
	// Name must match the name of one entry in pod.spec.resourceClaims of
	// the Pod where this field is used. It makes that resource available
	// inside a container.
	Name string `json:"name"`
}

func (in *ResourceClaim) DeepCopyInto(out *ResourceClaim) {
	*out = *in
}

func (in *ResourceClaim) DeepCopy() *ResourceClaim {
	if in == nil {
		return nil
	}
	out := new(ResourceClaim)
	in.DeepCopyInto(out)
	return out
}

type VolumeSource struct {
	// hostPath represents a pre-existing file or directory on the host
	// machine that is directly exposed to the container. This is generally
	// used for system agents or other privileged things that are allowed
	// to see the host machine. Most containers will NOT need this.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#hostpath
	// ---
	// TODO(jonesdl) We need to restrict who can use host directory mounts and who can/can not
	// mount host directories as read/write.
	HostPath *HostPathVolumeSource `json:"hostPath,omitempty"`
	// emptyDir represents a temporary directory that shares a pod's lifetime.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
	EmptyDir *EmptyDirVolumeSource `json:"emptyDir,omitempty"`
	// gcePersistentDisk represents a GCE Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#gcepersistentdisk
	GCEPersistentDisk *GCEPersistentDiskVolumeSource `json:"gcePersistentDisk,omitempty"`
	// awsElasticBlockStore represents an AWS Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#awselasticblockstore
	AWSElasticBlockStore *AWSElasticBlockStoreVolumeSource `json:"awsElasticBlockStore,omitempty"`
	// gitRepo represents a git repository at a particular revision.
	// DEPRECATED: GitRepo is deprecated. To provision a container with a git repo, mount an
	// EmptyDir into an InitContainer that clones the repo using git, then mount the EmptyDir
	// into the Pod's container.
	GitRepo *GitRepoVolumeSource `json:"gitRepo,omitempty"`
	// secret represents a secret that should populate this volume.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#secret
	Secret *SecretVolumeSource `json:"secret,omitempty"`
	// nfs represents an NFS mount on the host that shares a pod's lifetime
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#nfs
	NFS *NFSVolumeSource `json:"nfs,omitempty"`
	// iscsi represents an ISCSI Disk resource that is attached to a
	// kubelet's host machine and then exposed to the pod.
	// More info: https://examples.k8s.io/volumes/iscsi/README.md
	ISCSI *ISCSIVolumeSource `json:"iscsi,omitempty"`
	// glusterfs represents a Glusterfs mount on the host that shares a pod's lifetime.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md
	Glusterfs *GlusterfsVolumeSource `json:"glusterfs,omitempty"`
	// persistentVolumeClaimVolumeSource represents a reference to a
	// PersistentVolumeClaim in the same namespace.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	PersistentVolumeClaim *PersistentVolumeClaimVolumeSource `json:"persistentVolumeClaim,omitempty"`
	// rbd represents a Rados Block Device mount on the host that shares a pod's lifetime.
	// More info: https://examples.k8s.io/volumes/rbd/README.md
	RBD *RBDVolumeSource `json:"rbd,omitempty"`
	// flexVolume represents a generic volume resource that is
	// provisioned/attached using an exec based plugin.
	FlexVolume *FlexVolumeSource `json:"flexVolume,omitempty"`
	// cinder represents a cinder volume attached and mounted on kubelets host machine.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	Cinder *CinderVolumeSource `json:"cinder,omitempty"`
	// cephFS represents a Ceph FS mount on the host that shares a pod's lifetime
	CephFS *CephFSVolumeSource `json:"cephfs,omitempty"`
	// flocker represents a Flocker volume attached to a kubelet's host machine. This depends on the Flocker control service being running
	Flocker *FlockerVolumeSource `json:"flocker,omitempty"`
	// downwardAPI represents downward API about the pod that should populate this volume
	DownwardAPI *DownwardAPIVolumeSource `json:"downwardAPI,omitempty"`
	// fc represents a Fibre Channel resource that is attached to a kubelet's host machine and then exposed to the pod.
	FC *FCVolumeSource `json:"fc,omitempty"`
	// azureFile represents an Azure File Service mount on the host and bind mount to the pod.
	AzureFile *AzureFileVolumeSource `json:"azureFile,omitempty"`
	// configMap represents a configMap that should populate this volume
	ConfigMap *ConfigMapVolumeSource `json:"configMap,omitempty"`
	// vsphereVolume represents a vSphere volume attached and mounted on kubelets host machine
	VsphereVolume *VsphereVirtualDiskVolumeSource `json:"vsphereVolume,omitempty"`
	// quobyte represents a Quobyte mount on the host that shares a pod's lifetime
	Quobyte *QuobyteVolumeSource `json:"quobyte,omitempty"`
	// azureDisk represents an Azure Data Disk mount on the host and bind mount to the pod.
	AzureDisk *AzureDiskVolumeSource `json:"azureDisk,omitempty"`
	// photonPersistentDisk represents a PhotonController persistent disk attached and mounted on kubelets host machine
	PhotonPersistentDisk *PhotonPersistentDiskVolumeSource `json:"photonPersistentDisk,omitempty"`
	// projected items for all in one resources secrets, configmaps, and downward API
	Projected *ProjectedVolumeSource `json:"projected,omitempty"`
	// portworxVolume represents a portworx volume attached and mounted on kubelets host machine
	PortworxVolume *PortworxVolumeSource `json:"portworxVolume,omitempty"`
	// scaleIO represents a ScaleIO persistent volume attached and mounted on Kubernetes nodes.
	ScaleIO *ScaleIOVolumeSource `json:"scaleIO,omitempty"`
	// storageOS represents a StorageOS volume attached and mounted on Kubernetes nodes.
	StorageOS *StorageOSVolumeSource `json:"storageos,omitempty"`
	// csi (Container Storage Interface) represents ephemeral storage that is handled by certain external CSI drivers (Beta feature).
	CSI *CSIVolumeSource `json:"csi,omitempty"`
	// ephemeral represents a volume that is handled by a cluster storage driver.
	// The volume's lifecycle is tied to the pod that defines it - it will be created before the pod starts,
	// and deleted when the pod is removed.
	// Use this if:
	// a) the volume is only needed while the pod runs,
	// b) features of normal volumes like restoring from snapshot or capacity
	// tracking are needed,
	// c) the storage driver is specified through a storage class, and
	// d) the storage driver supports dynamic volume provisioning through
	// a PersistentVolumeClaim (see EphemeralVolumeSource for more
	// information on the connection between this volume type
	// and PersistentVolumeClaim).
	// Use PersistentVolumeClaim or one of the vendor-specific
	// APIs for volumes that persist for longer than the lifecycle
	// of an individual pod.
	// Use CSI for light-weight local ephemeral volumes if the CSI driver is meant to
	// be used that way - see the documentation of the driver for
	// more information.
	// A pod can use both types of ephemeral volumes and
	// persistent volumes at the same time.
	Ephemeral *EphemeralVolumeSource `json:"ephemeral,omitempty"`
}

func (in *VolumeSource) DeepCopyInto(out *VolumeSource) {
	*out = *in
	if in.HostPath != nil {
		in, out := &in.HostPath, &out.HostPath
		*out = new(HostPathVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.EmptyDir != nil {
		in, out := &in.EmptyDir, &out.EmptyDir
		*out = new(EmptyDirVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.GCEPersistentDisk != nil {
		in, out := &in.GCEPersistentDisk, &out.GCEPersistentDisk
		*out = new(GCEPersistentDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.AWSElasticBlockStore != nil {
		in, out := &in.AWSElasticBlockStore, &out.AWSElasticBlockStore
		*out = new(AWSElasticBlockStoreVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.GitRepo != nil {
		in, out := &in.GitRepo, &out.GitRepo
		*out = new(GitRepoVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(SecretVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.NFS != nil {
		in, out := &in.NFS, &out.NFS
		*out = new(NFSVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.ISCSI != nil {
		in, out := &in.ISCSI, &out.ISCSI
		*out = new(ISCSIVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Glusterfs != nil {
		in, out := &in.Glusterfs, &out.Glusterfs
		*out = new(GlusterfsVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.PersistentVolumeClaim != nil {
		in, out := &in.PersistentVolumeClaim, &out.PersistentVolumeClaim
		*out = new(PersistentVolumeClaimVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.RBD != nil {
		in, out := &in.RBD, &out.RBD
		*out = new(RBDVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.FlexVolume != nil {
		in, out := &in.FlexVolume, &out.FlexVolume
		*out = new(FlexVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Cinder != nil {
		in, out := &in.Cinder, &out.Cinder
		*out = new(CinderVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.CephFS != nil {
		in, out := &in.CephFS, &out.CephFS
		*out = new(CephFSVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Flocker != nil {
		in, out := &in.Flocker, &out.Flocker
		*out = new(FlockerVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.DownwardAPI != nil {
		in, out := &in.DownwardAPI, &out.DownwardAPI
		*out = new(DownwardAPIVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.FC != nil {
		in, out := &in.FC, &out.FC
		*out = new(FCVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.AzureFile != nil {
		in, out := &in.AzureFile, &out.AzureFile
		*out = new(AzureFileVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.ConfigMap != nil {
		in, out := &in.ConfigMap, &out.ConfigMap
		*out = new(ConfigMapVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.VsphereVolume != nil {
		in, out := &in.VsphereVolume, &out.VsphereVolume
		*out = new(VsphereVirtualDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Quobyte != nil {
		in, out := &in.Quobyte, &out.Quobyte
		*out = new(QuobyteVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.AzureDisk != nil {
		in, out := &in.AzureDisk, &out.AzureDisk
		*out = new(AzureDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.PhotonPersistentDisk != nil {
		in, out := &in.PhotonPersistentDisk, &out.PhotonPersistentDisk
		*out = new(PhotonPersistentDiskVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Projected != nil {
		in, out := &in.Projected, &out.Projected
		*out = new(ProjectedVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.PortworxVolume != nil {
		in, out := &in.PortworxVolume, &out.PortworxVolume
		*out = new(PortworxVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.ScaleIO != nil {
		in, out := &in.ScaleIO, &out.ScaleIO
		*out = new(ScaleIOVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.StorageOS != nil {
		in, out := &in.StorageOS, &out.StorageOS
		*out = new(StorageOSVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.CSI != nil {
		in, out := &in.CSI, &out.CSI
		*out = new(CSIVolumeSource)
		(*in).DeepCopyInto(*out)
	}
	if in.Ephemeral != nil {
		in, out := &in.Ephemeral, &out.Ephemeral
		*out = new(EphemeralVolumeSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *VolumeSource) DeepCopy() *VolumeSource {
	if in == nil {
		return nil
	}
	out := new(VolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ContainerPort struct {
	// If specified, this must be an IANA_SVC_NAME and unique within the pod. Each
	// named port in a pod must have a unique name. Name for the port that can be
	// referred to by services.
	Name string `json:"name,omitempty"`
	// Number of port to expose on the host.
	// If specified, this must be a valid port number, 0 < x < 65536.
	// If HostNetwork is specified, this must match ContainerPort.
	// Most containers do not need this.
	HostPort int `json:"hostPort,omitempty"`
	// Number of port to expose on the pod's IP address.
	// This must be a valid port number, 0 < x < 65536.
	ContainerPort int `json:"containerPort"`
	// Protocol for port. Must be UDP, TCP, or SCTP.
	// Defaults to "TCP".
	Protocol Protocol `json:"protocol,omitempty"`
	// What host IP to bind the external port to.
	HostIP string `json:"hostIP,omitempty"`
}

func (in *ContainerPort) DeepCopyInto(out *ContainerPort) {
	*out = *in
}

func (in *ContainerPort) DeepCopy() *ContainerPort {
	if in == nil {
		return nil
	}
	out := new(ContainerPort)
	in.DeepCopyInto(out)
	return out
}

type EnvFromSource struct {
	// An optional identifier to prepend to each key in the ConfigMap. Must be a C_IDENTIFIER.
	Prefix string `json:"prefix,omitempty"`
	// The ConfigMap to select from
	ConfigMapRef *ConfigMapEnvSource `json:"configMapRef,omitempty"`
	// The Secret to select from
	SecretRef *SecretEnvSource `json:"secretRef,omitempty"`
}

func (in *EnvFromSource) DeepCopyInto(out *EnvFromSource) {
	*out = *in
	if in.ConfigMapRef != nil {
		in, out := &in.ConfigMapRef, &out.ConfigMapRef
		*out = new(ConfigMapEnvSource)
		(*in).DeepCopyInto(*out)
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(SecretEnvSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EnvFromSource) DeepCopy() *EnvFromSource {
	if in == nil {
		return nil
	}
	out := new(EnvFromSource)
	in.DeepCopyInto(out)
	return out
}

type EnvVar struct {
	// Name of the environment variable. Must be a C_IDENTIFIER.
	Name string `json:"name"`
	// Variable references $(VAR_NAME) are expanded
	// using the previously defined environment variables in the container and
	// any service environment variables. If a variable cannot be resolved,
	// the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e.
	// "$$(VAR_NAME)" will produce the string literal "$(VAR_NAME)".
	// Escaped references will never be expanded, regardless of whether the variable
	// exists or not.
	// Defaults to "".
	Value string `json:"value,omitempty"`
	// Source for the environment variable's value. Cannot be used if value is not empty.
	ValueFrom *EnvVarSource `json:"valueFrom,omitempty"`
}

func (in *EnvVar) DeepCopyInto(out *EnvVar) {
	*out = *in
	if in.ValueFrom != nil {
		in, out := &in.ValueFrom, &out.ValueFrom
		*out = new(EnvVarSource)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EnvVar) DeepCopy() *EnvVar {
	if in == nil {
		return nil
	}
	out := new(EnvVar)
	in.DeepCopyInto(out)
	return out
}

type ContainerResizePolicy struct {
	// Name of the resource to which this resource resize policy applies.
	// Supported values: cpu, memory.
	ResourceName ResourceName `json:"resourceName"`
	// Restart policy to apply when specified resource is resized.
	// If not specified, it defaults to NotRequired.
	RestartPolicy ResourceResizeRestartPolicy `json:"restartPolicy"`
}

func (in *ContainerResizePolicy) DeepCopyInto(out *ContainerResizePolicy) {
	*out = *in
}

func (in *ContainerResizePolicy) DeepCopy() *ContainerResizePolicy {
	if in == nil {
		return nil
	}
	out := new(ContainerResizePolicy)
	in.DeepCopyInto(out)
	return out
}

type VolumeMount struct {
	// This must match the Name of a Volume.
	Name string `json:"name"`
	// Mounted read-only if true, read-write otherwise (false or unspecified).
	// Defaults to false.
	ReadOnly bool `json:"readOnly,omitempty"`
	// Path within the container at which the volume should be mounted.  Must
	// not contain ':'.
	MountPath string `json:"mountPath"`
	// Path within the volume from which the container's volume should be mounted.
	// Defaults to "" (volume's root).
	SubPath string `json:"subPath,omitempty"`
	// mountPropagation determines how mounts are propagated from the host
	// to container and the other way around.
	// When not set, MountPropagationNone is used.
	// This field is beta in 1.10.
	MountPropagation MountPropagationMode `json:"mountPropagation,omitempty"`
	// Expanded path within the volume from which the container's volume should be mounted.
	// Behaves similarly to SubPath but environment variable references $(VAR_NAME) are expanded using the container's environment.
	// Defaults to "" (volume's root).
	// SubPathExpr and SubPath are mutually exclusive.
	SubPathExpr string `json:"subPathExpr,omitempty"`
}

func (in *VolumeMount) DeepCopyInto(out *VolumeMount) {
	*out = *in
}

func (in *VolumeMount) DeepCopy() *VolumeMount {
	if in == nil {
		return nil
	}
	out := new(VolumeMount)
	in.DeepCopyInto(out)
	return out
}

type VolumeDevice struct {
	// name must match the name of a persistentVolumeClaim in the pod
	Name string `json:"name"`
	// devicePath is the path inside of the container that the device will be mapped to.
	DevicePath string `json:"devicePath"`
}

func (in *VolumeDevice) DeepCopyInto(out *VolumeDevice) {
	*out = *in
}

func (in *VolumeDevice) DeepCopy() *VolumeDevice {
	if in == nil {
		return nil
	}
	out := new(VolumeDevice)
	in.DeepCopyInto(out)
	return out
}

type Probe struct {
	// The action taken to determine the health of a container
	ProbeHandler `json:",inline"`
	// Number of seconds after the container has started before liveness probes are initiated.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	InitialDelaySeconds int `json:"initialDelaySeconds,omitempty"`
	// Number of seconds after which the probe times out.
	// Defaults to 1 second. Minimum value is 1.
	// More info: https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle#container-probes
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
	// How often (in seconds) to perform the probe.
	// Default to 10 seconds. Minimum value is 1.
	PeriodSeconds int `json:"periodSeconds,omitempty"`
	// Minimum consecutive successes for the probe to be considered successful after having failed.
	// Defaults to 1. Must be 1 for liveness and startup. Minimum value is 1.
	SuccessThreshold int `json:"successThreshold,omitempty"`
	// Minimum consecutive failures for the probe to be considered failed after having succeeded.
	// Defaults to 3. Minimum value is 1.
	FailureThreshold int `json:"failureThreshold,omitempty"`
	// Optional duration in seconds the pod needs to terminate gracefully upon probe failure.
	// The grace period is the duration in seconds after the processes running in the pod are sent
	// a termination signal and the time when the processes are forcibly halted with a kill signal.
	// Set this value longer than the expected cleanup time for your process.
	// If this value is nil, the pod's terminationGracePeriodSeconds will be used. Otherwise, this
	// value overrides the value provided by the pod spec.
	// Value must be non-negative integer. The value zero indicates stop immediately via
	// the kill signal (no opportunity to shut down).
	// This is a beta field and requires enabling ProbeTerminationGracePeriod feature gate.
	// Minimum value is 1. spec.terminationGracePeriodSeconds is used if unset.
	TerminationGracePeriodSeconds int64 `json:"terminationGracePeriodSeconds,omitempty"`
}

func (in *Probe) DeepCopyInto(out *Probe) {
	*out = *in
	out.ProbeHandler = in.ProbeHandler
}

func (in *Probe) DeepCopy() *Probe {
	if in == nil {
		return nil
	}
	out := new(Probe)
	in.DeepCopyInto(out)
	return out
}

type Lifecycle struct {
	// PostStart is called immediately after a container is created. If the handler fails,
	// the container is terminated and restarted according to its restart policy.
	// Other management of the container blocks until the hook completes.
	// More info: https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks
	PostStart *LifecycleHandler `json:"postStart,omitempty"`
	// PreStop is called immediately before a container is terminated due to an
	// API request or management event such as liveness/startup probe failure,
	// preemption, resource contention, etc. The handler is not called if the
	// container crashes or exits. The Pod's termination grace period countdown begins before the
	// PreStop hook is executed. Regardless of the outcome of the handler, the
	// container will eventually terminate within the Pod's termination grace
	// period (unless delayed by finalizers). Other management of the container blocks until the hook completes
	// or until the termination grace period is reached.
	// More info: https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/#container-hooks
	PreStop *LifecycleHandler `json:"preStop,omitempty"`
}

func (in *Lifecycle) DeepCopyInto(out *Lifecycle) {
	*out = *in
	if in.PostStart != nil {
		in, out := &in.PostStart, &out.PostStart
		*out = new(LifecycleHandler)
		(*in).DeepCopyInto(*out)
	}
	if in.PreStop != nil {
		in, out := &in.PreStop, &out.PreStop
		*out = new(LifecycleHandler)
		(*in).DeepCopyInto(*out)
	}
}

func (in *Lifecycle) DeepCopy() *Lifecycle {
	if in == nil {
		return nil
	}
	out := new(Lifecycle)
	in.DeepCopyInto(out)
	return out
}

type SecurityContext struct {
	// The capabilities to add/drop when running containers.
	// Defaults to the default set of capabilities granted by the container runtime.
	// Note that this field cannot be set when spec.os.name is windows.
	Capabilities *Capabilities `json:"capabilities,omitempty"`
	// Run container in privileged mode.
	// Processes in privileged containers are essentially equivalent to root on the host.
	// Defaults to false.
	// Note that this field cannot be set when spec.os.name is windows.
	Privileged bool `json:"privileged,omitempty"`
	// The SELinux context to be applied to the container.
	// If unspecified, the container runtime will allocate a random SELinux context for each
	// container.  May also be set in PodSecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence.
	// Note that this field cannot be set when spec.os.name is windows.
	SELinuxOptions *SELinuxOptions `json:"seLinuxOptions,omitempty"`
	// The Windows specific settings applied to all containers.
	// If unspecified, the options from the PodSecurityContext will be used.
	// If set in both SecurityContext and PodSecurityContext, the value specified in SecurityContext takes precedence.
	// Note that this field cannot be set when spec.os.name is linux.
	WindowsOptions *WindowsSecurityContextOptions `json:"windowsOptions,omitempty"`
	// The UID to run the entrypoint of the container process.
	// Defaults to user specified in image metadata if unspecified.
	// May also be set in PodSecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence.
	// Note that this field cannot be set when spec.os.name is windows.
	RunAsUser int64 `json:"runAsUser,omitempty"`
	// The GID to run the entrypoint of the container process.
	// Uses runtime default if unset.
	// May also be set in PodSecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence.
	// Note that this field cannot be set when spec.os.name is windows.
	RunAsGroup int64 `json:"runAsGroup,omitempty"`
	// Indicates that the container must run as a non-root user.
	// If true, the Kubelet will validate the image at runtime to ensure that it
	// does not run as UID 0 (root) and fail to start the container if it does.
	// If unset or false, no such validation will be performed.
	// May also be set in PodSecurityContext.  If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence.
	RunAsNonRoot bool `json:"runAsNonRoot,omitempty"`
	// Whether this container has a read-only root filesystem.
	// Default is false.
	// Note that this field cannot be set when spec.os.name is windows.
	ReadOnlyRootFilesystem bool `json:"readOnlyRootFilesystem,omitempty"`
	// AllowPrivilegeEscalation controls whether a process can gain more
	// privileges than its parent process. This bool directly controls if
	// the no_new_privs flag will be set on the container process.
	// AllowPrivilegeEscalation is true always when the container is:
	// 1) run as Privileged
	// 2) has CAP_SYS_ADMIN
	// Note that this field cannot be set when spec.os.name is windows.
	AllowPrivilegeEscalation bool `json:"allowPrivilegeEscalation,omitempty"`
	// procMount denotes the type of proc mount to use for the containers.
	// The default is DefaultProcMount which uses the container runtime defaults for
	// readonly paths and masked paths.
	// This requires the ProcMountType feature flag to be enabled.
	// Note that this field cannot be set when spec.os.name is windows.
	ProcMount ProcMountType `json:"procMount,omitempty"`
	// The seccomp options to use by this container. If seccomp options are
	// provided at both the pod & container level, the container options
	// override the pod options.
	// Note that this field cannot be set when spec.os.name is windows.
	SeccompProfile *SeccompProfile `json:"seccompProfile,omitempty"`
}

func (in *SecurityContext) DeepCopyInto(out *SecurityContext) {
	*out = *in
	if in.Capabilities != nil {
		in, out := &in.Capabilities, &out.Capabilities
		*out = new(Capabilities)
		(*in).DeepCopyInto(*out)
	}
	if in.SELinuxOptions != nil {
		in, out := &in.SELinuxOptions, &out.SELinuxOptions
		*out = new(SELinuxOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.WindowsOptions != nil {
		in, out := &in.WindowsOptions, &out.WindowsOptions
		*out = new(WindowsSecurityContextOptions)
		(*in).DeepCopyInto(*out)
	}
	if in.SeccompProfile != nil {
		in, out := &in.SeccompProfile, &out.SeccompProfile
		*out = new(SeccompProfile)
		(*in).DeepCopyInto(*out)
	}
}

func (in *SecurityContext) DeepCopy() *SecurityContext {
	if in == nil {
		return nil
	}
	out := new(SecurityContext)
	in.DeepCopyInto(out)
	return out
}

type EphemeralContainerCommon struct {
	// Name of the ephemeral container specified as a DNS_LABEL.
	// This name must be unique among all containers, init containers and ephemeral containers.
	Name string `json:"name"`
	// Container image name.
	// More info: https://kubernetes.io/docs/concepts/containers/images
	Image string `json:"image,omitempty"`
	// Entrypoint array. Not executed within a shell.
	// The image's ENTRYPOINT is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	Command []string `json:"command"`
	// Arguments to the entrypoint.
	// The image's CMD is used if this is not provided.
	// Variable references $(VAR_NAME) are expanded using the container's environment. If a variable
	// cannot be resolved, the reference in the input string will be unchanged. Double $$ are reduced
	// to a single $, which allows for escaping the $(VAR_NAME) syntax: i.e. "$$(VAR_NAME)" will
	// produce the string literal "$(VAR_NAME)". Escaped references will never be expanded, regardless
	// of whether the variable exists or not. Cannot be updated.
	// More info: https://kubernetes.io/docs/tasks/inject-data-application/define-command-argument-container/#running-a-command-in-a-shell
	Args []string `json:"args"`
	// Container's working directory.
	// If not specified, the container runtime's default will be used, which
	// might be configured in the container image.
	// Cannot be updated.
	WorkingDir string `json:"workingDir,omitempty"`
	// Ports are not allowed for ephemeral containers.
	Ports []ContainerPort `json:"ports"`
	// List of sources to populate environment variables in the container.
	// The keys defined within a source must be a C_IDENTIFIER. All invalid keys
	// will be reported as an event when the container is starting. When a key exists in multiple
	// sources, the value associated with the last source will take precedence.
	// Values defined by an Env with a duplicate key will take precedence.
	// Cannot be updated.
	EnvFrom []EnvFromSource `json:"envFrom"`
	// List of environment variables to set in the container.
	// Cannot be updated.
	Env []EnvVar `json:"env"`
	// Resources are not allowed for ephemeral containers. Ephemeral containers use spare resources
	// already allocated to the pod.
	Resources *ResourceRequirements `json:"resources,omitempty"`
	// Resources resize policy for the container.
	ResizePolicy []ContainerResizePolicy `json:"resizePolicy"`
	// Pod volumes to mount into the container's filesystem. Subpath mounts are not allowed for ephemeral containers.
	// Cannot be updated.
	VolumeMounts []VolumeMount `json:"volumeMounts"`
	// volumeDevices is the list of block devices to be used by the container.
	VolumeDevices []VolumeDevice `json:"volumeDevices"`
	// Probes are not allowed for ephemeral containers.
	LivenessProbe *Probe `json:"livenessProbe,omitempty"`
	// Probes are not allowed for ephemeral containers.
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
	// Probes are not allowed for ephemeral containers.
	StartupProbe *Probe `json:"startupProbe,omitempty"`
	// Lifecycle is not allowed for ephemeral containers.
	Lifecycle *Lifecycle `json:"lifecycle,omitempty"`
	// Optional: Path at which the file to which the container's termination message
	// will be written is mounted into the container's filesystem.
	// Message written is intended to be brief final status, such as an assertion failure message.
	// Will be truncated by the node if greater than 4096 bytes. The total message length across
	// all containers will be limited to 12kb.
	// Defaults to /dev/termination-log.
	// Cannot be updated.
	TerminationMessagePath string `json:"terminationMessagePath,omitempty"`
	// Indicate how the termination message should be populated. File will use the contents of
	// terminationMessagePath to populate the container status message on both success and failure.
	// FallbackToLogsOnError will use the last chunk of container log output if the termination
	// message file is empty and the container exited with an error.
	// The log output is limited to 2048 bytes or 80 lines, whichever is smaller.
	// Defaults to File.
	// Cannot be updated.
	TerminationMessagePolicy TerminationMessagePolicy `json:"terminationMessagePolicy,omitempty"`
	// Image pull policy.
	// One of Always, Never, IfNotPresent.
	// Defaults to Always if :latest tag is specified, or IfNotPresent otherwise.
	// Cannot be updated.
	// More info: https://kubernetes.io/docs/concepts/containers/images#updating-images
	ImagePullPolicy PullPolicy `json:"imagePullPolicy,omitempty"`
	// Optional: SecurityContext defines the security options the ephemeral container should be run with.
	// If set, the fields of SecurityContext override the equivalent fields of PodSecurityContext.
	SecurityContext *SecurityContext `json:"securityContext,omitempty"`
	// Whether this container should allocate a buffer for stdin in the container runtime. If this
	// is not set, reads from stdin in the container will always result in EOF.
	// Default is false.
	Stdin bool `json:"stdin,omitempty"`
	// Whether the container runtime should close the stdin channel after it has been opened by
	// a single attach. When stdin is true the stdin stream will remain open across multiple attach
	// sessions. If stdinOnce is set to true, stdin is opened on container start, is empty until the
	// first client attaches to stdin, and then remains open and accepts data until the client disconnects,
	// at which time stdin is closed and remains closed until the container is restarted. If this
	// flag is false, a container processes that reads from stdin will never receive an EOF.
	// Default is false
	StdinOnce bool `json:"stdinOnce,omitempty"`
	// Whether this container should allocate a TTY for itself, also requires 'stdin' to be true.
	// Default is false.
	TTY bool `json:"tty,omitempty"`
}

func (in *EphemeralContainerCommon) DeepCopyInto(out *EphemeralContainerCommon) {
	*out = *in
	if in.Command != nil {
		t := make([]string, len(in.Command))
		copy(t, in.Command)
		out.Command = t
	}
	if in.Args != nil {
		t := make([]string, len(in.Args))
		copy(t, in.Args)
		out.Args = t
	}
	if in.Ports != nil {
		l := make([]ContainerPort, len(in.Ports))
		for i := range in.Ports {
			in.Ports[i].DeepCopyInto(&l[i])
		}
		out.Ports = l
	}
	if in.EnvFrom != nil {
		l := make([]EnvFromSource, len(in.EnvFrom))
		for i := range in.EnvFrom {
			in.EnvFrom[i].DeepCopyInto(&l[i])
		}
		out.EnvFrom = l
	}
	if in.Env != nil {
		l := make([]EnvVar, len(in.Env))
		for i := range in.Env {
			in.Env[i].DeepCopyInto(&l[i])
		}
		out.Env = l
	}
	if in.Resources != nil {
		in, out := &in.Resources, &out.Resources
		*out = new(ResourceRequirements)
		(*in).DeepCopyInto(*out)
	}
	if in.ResizePolicy != nil {
		l := make([]ContainerResizePolicy, len(in.ResizePolicy))
		for i := range in.ResizePolicy {
			in.ResizePolicy[i].DeepCopyInto(&l[i])
		}
		out.ResizePolicy = l
	}
	if in.VolumeMounts != nil {
		l := make([]VolumeMount, len(in.VolumeMounts))
		for i := range in.VolumeMounts {
			in.VolumeMounts[i].DeepCopyInto(&l[i])
		}
		out.VolumeMounts = l
	}
	if in.VolumeDevices != nil {
		l := make([]VolumeDevice, len(in.VolumeDevices))
		for i := range in.VolumeDevices {
			in.VolumeDevices[i].DeepCopyInto(&l[i])
		}
		out.VolumeDevices = l
	}
	if in.LivenessProbe != nil {
		in, out := &in.LivenessProbe, &out.LivenessProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.ReadinessProbe != nil {
		in, out := &in.ReadinessProbe, &out.ReadinessProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.StartupProbe != nil {
		in, out := &in.StartupProbe, &out.StartupProbe
		*out = new(Probe)
		(*in).DeepCopyInto(*out)
	}
	if in.Lifecycle != nil {
		in, out := &in.Lifecycle, &out.Lifecycle
		*out = new(Lifecycle)
		(*in).DeepCopyInto(*out)
	}
	if in.SecurityContext != nil {
		in, out := &in.SecurityContext, &out.SecurityContext
		*out = new(SecurityContext)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EphemeralContainerCommon) DeepCopy() *EphemeralContainerCommon {
	if in == nil {
		return nil
	}
	out := new(EphemeralContainerCommon)
	in.DeepCopyInto(out)
	return out
}

type SELinuxOptions struct {
	// User is a SELinux user label that applies to the container.
	User string `json:"user,omitempty"`
	// Role is a SELinux role label that applies to the container.
	Role string `json:"role,omitempty"`
	// Type is a SELinux type label that applies to the container.
	Type string `json:"type,omitempty"`
	// Level is SELinux level label that applies to the container.
	Level string `json:"level,omitempty"`
}

func (in *SELinuxOptions) DeepCopyInto(out *SELinuxOptions) {
	*out = *in
}

func (in *SELinuxOptions) DeepCopy() *SELinuxOptions {
	if in == nil {
		return nil
	}
	out := new(SELinuxOptions)
	in.DeepCopyInto(out)
	return out
}

type WindowsSecurityContextOptions struct {
	// GMSACredentialSpecName is the name of the GMSA credential spec to use.
	GMSACredentialSpecName string `json:"gmsaCredentialSpecName,omitempty"`
	// GMSACredentialSpec is where the GMSA admission webhook
	// (https://github.com/kubernetes-sigs/windows-gmsa) inlines the contents of the
	// GMSA credential spec named by the GMSACredentialSpecName field.
	GMSACredentialSpec string `json:"gmsaCredentialSpec,omitempty"`
	// The UserName in Windows to run the entrypoint of the container process.
	// Defaults to the user specified in image metadata if unspecified.
	// May also be set in PodSecurityContext. If set in both SecurityContext and
	// PodSecurityContext, the value specified in SecurityContext takes precedence.
	RunAsUserName string `json:"runAsUserName,omitempty"`
	// HostProcess determines if a container should be run as a 'Host Process' container.
	// This field is alpha-level and will only be honored by components that enable the
	// WindowsHostProcessContainers feature flag. Setting this field without the feature
	// flag will result in errors when validating the Pod. All of a Pod's containers must
	// have the same effective HostProcess value (it is not allowed to have a mix of HostProcess
	// containers and non-HostProcess containers).  In addition, if HostProcess is true
	// then HostNetwork must also be set to true.
	HostProcess bool `json:"hostProcess,omitempty"`
}

func (in *WindowsSecurityContextOptions) DeepCopyInto(out *WindowsSecurityContextOptions) {
	*out = *in
}

func (in *WindowsSecurityContextOptions) DeepCopy() *WindowsSecurityContextOptions {
	if in == nil {
		return nil
	}
	out := new(WindowsSecurityContextOptions)
	in.DeepCopyInto(out)
	return out
}

type Sysctl struct {
	// Name of a property to set
	Name string `json:"name"`
	// Value of a property to set
	Value string `json:"value"`
}

func (in *Sysctl) DeepCopyInto(out *Sysctl) {
	*out = *in
}

func (in *Sysctl) DeepCopy() *Sysctl {
	if in == nil {
		return nil
	}
	out := new(Sysctl)
	in.DeepCopyInto(out)
	return out
}

type SeccompProfile struct {
	// type indicates which kind of seccomp profile will be applied.
	// Valid options are:
	// Localhost - a profile defined in a file on the node should be used.
	// RuntimeDefault - the container runtime default profile should be used.
	// Unconfined - no profile should be applied.
	Type SeccompProfileType `json:"type"`
	// localhostProfile indicates a profile defined in a file on the node should be used.
	// The profile must be preconfigured on the node to work.
	// Must be a descending path, relative to the kubelet's configured seccomp profile location.
	// Must only be set if type is "Localhost".
	LocalhostProfile string `json:"localhostProfile,omitempty"`
}

func (in *SeccompProfile) DeepCopyInto(out *SeccompProfile) {
	*out = *in
}

func (in *SeccompProfile) DeepCopy() *SeccompProfile {
	if in == nil {
		return nil
	}
	out := new(SeccompProfile)
	in.DeepCopyInto(out)
	return out
}

type NodeAffinity struct {
	// If the affinity requirements specified by this field are not met at
	// scheduling time, the pod will not be scheduled onto the node.
	// If the affinity requirements specified by this field cease to be met
	// at some point during pod execution (e.g. due to an update), the system
	// may or may not try to eventually evict the pod from its node.
	RequiredDuringSchedulingIgnoredDuringExecution *NodeSelector `json:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	// The scheduler will prefer to schedule pods to nodes that satisfy
	// the affinity expressions specified by this field, but it may choose
	// a node that violates one or more of the expressions. The node that is
	// most preferred is the one with the greatest sum of weights, i.e.
	// for each node that meets all of the scheduling requirements (resource
	// request, requiredDuringScheduling affinity expressions, etc.),
	// compute a sum by iterating through the elements of this field and adding
	// "weight" to the sum if the node matches the corresponding matchExpressions; the
	// node(s) with the highest sum are the most preferred.
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `json:"preferredDuringSchedulingIgnoredDuringExecution"`
}

func (in *NodeAffinity) DeepCopyInto(out *NodeAffinity) {
	*out = *in
	if in.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		in, out := &in.RequiredDuringSchedulingIgnoredDuringExecution, &out.RequiredDuringSchedulingIgnoredDuringExecution
		*out = new(NodeSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		l := make([]PreferredSchedulingTerm, len(in.PreferredDuringSchedulingIgnoredDuringExecution))
		for i := range in.PreferredDuringSchedulingIgnoredDuringExecution {
			in.PreferredDuringSchedulingIgnoredDuringExecution[i].DeepCopyInto(&l[i])
		}
		out.PreferredDuringSchedulingIgnoredDuringExecution = l
	}
}

func (in *NodeAffinity) DeepCopy() *NodeAffinity {
	if in == nil {
		return nil
	}
	out := new(NodeAffinity)
	in.DeepCopyInto(out)
	return out
}

type PodAffinity struct {
	// If the affinity requirements specified by this field are not met at
	// scheduling time, the pod will not be scheduled onto the node.
	// If the affinity requirements specified by this field cease to be met
	// at some point during pod execution (e.g. due to a pod label update), the
	// system may or may not try to eventually evict the pod from its node.
	// When there are multiple elements, the lists of nodes corresponding to each
	// podAffinityTerm are intersected, i.e. all terms must be satisfied.
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution"`
	// The scheduler will prefer to schedule pods to nodes that satisfy
	// the affinity expressions specified by this field, but it may choose
	// a node that violates one or more of the expressions. The node that is
	// most preferred is the one with the greatest sum of weights, i.e.
	// for each node that meets all of the scheduling requirements (resource
	// request, requiredDuringScheduling affinity expressions, etc.),
	// compute a sum by iterating through the elements of this field and adding
	// "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the
	// node(s) with the highest sum are the most preferred.
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution"`
}

func (in *PodAffinity) DeepCopyInto(out *PodAffinity) {
	*out = *in
	if in.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		l := make([]PodAffinityTerm, len(in.RequiredDuringSchedulingIgnoredDuringExecution))
		for i := range in.RequiredDuringSchedulingIgnoredDuringExecution {
			in.RequiredDuringSchedulingIgnoredDuringExecution[i].DeepCopyInto(&l[i])
		}
		out.RequiredDuringSchedulingIgnoredDuringExecution = l
	}
	if in.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		l := make([]WeightedPodAffinityTerm, len(in.PreferredDuringSchedulingIgnoredDuringExecution))
		for i := range in.PreferredDuringSchedulingIgnoredDuringExecution {
			in.PreferredDuringSchedulingIgnoredDuringExecution[i].DeepCopyInto(&l[i])
		}
		out.PreferredDuringSchedulingIgnoredDuringExecution = l
	}
}

func (in *PodAffinity) DeepCopy() *PodAffinity {
	if in == nil {
		return nil
	}
	out := new(PodAffinity)
	in.DeepCopyInto(out)
	return out
}

type PodAntiAffinity struct {
	// If the anti-affinity requirements specified by this field are not met at
	// scheduling time, the pod will not be scheduled onto the node.
	// If the anti-affinity requirements specified by this field cease to be met
	// at some point during pod execution (e.g. due to a pod label update), the
	// system may or may not try to eventually evict the pod from its node.
	// When there are multiple elements, the lists of nodes corresponding to each
	// podAffinityTerm are intersected, i.e. all terms must be satisfied.
	RequiredDuringSchedulingIgnoredDuringExecution []PodAffinityTerm `json:"requiredDuringSchedulingIgnoredDuringExecution"`
	// The scheduler will prefer to schedule pods to nodes that satisfy
	// the anti-affinity expressions specified by this field, but it may choose
	// a node that violates one or more of the expressions. The node that is
	// most preferred is the one with the greatest sum of weights, i.e.
	// for each node that meets all of the scheduling requirements (resource
	// request, requiredDuringScheduling anti-affinity expressions, etc.),
	// compute a sum by iterating through the elements of this field and adding
	// "weight" to the sum if the node has pods which matches the corresponding podAffinityTerm; the
	// node(s) with the highest sum are the most preferred.
	PreferredDuringSchedulingIgnoredDuringExecution []WeightedPodAffinityTerm `json:"preferredDuringSchedulingIgnoredDuringExecution"`
}

func (in *PodAntiAffinity) DeepCopyInto(out *PodAntiAffinity) {
	*out = *in
	if in.RequiredDuringSchedulingIgnoredDuringExecution != nil {
		l := make([]PodAffinityTerm, len(in.RequiredDuringSchedulingIgnoredDuringExecution))
		for i := range in.RequiredDuringSchedulingIgnoredDuringExecution {
			in.RequiredDuringSchedulingIgnoredDuringExecution[i].DeepCopyInto(&l[i])
		}
		out.RequiredDuringSchedulingIgnoredDuringExecution = l
	}
	if in.PreferredDuringSchedulingIgnoredDuringExecution != nil {
		l := make([]WeightedPodAffinityTerm, len(in.PreferredDuringSchedulingIgnoredDuringExecution))
		for i := range in.PreferredDuringSchedulingIgnoredDuringExecution {
			in.PreferredDuringSchedulingIgnoredDuringExecution[i].DeepCopyInto(&l[i])
		}
		out.PreferredDuringSchedulingIgnoredDuringExecution = l
	}
}

func (in *PodAntiAffinity) DeepCopy() *PodAntiAffinity {
	if in == nil {
		return nil
	}
	out := new(PodAntiAffinity)
	in.DeepCopyInto(out)
	return out
}

type PodDNSConfigOption struct {
	// Required.
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

func (in *PodDNSConfigOption) DeepCopyInto(out *PodDNSConfigOption) {
	*out = *in
}

func (in *PodDNSConfigOption) DeepCopy() *PodDNSConfigOption {
	if in == nil {
		return nil
	}
	out := new(PodDNSConfigOption)
	in.DeepCopyInto(out)
	return out
}

type ClaimSource struct {
	// ResourceClaimName is the name of a ResourceClaim object in the same
	// namespace as this pod.
	ResourceClaimName string `json:"resourceClaimName,omitempty"`
	// ResourceClaimTemplateName is the name of a ResourceClaimTemplate
	// object in the same namespace as this pod.
	// The template will be used to create a new ResourceClaim, which will
	// be bound to this pod. When this pod is deleted, the ResourceClaim
	// will also be deleted. The name of the ResourceClaim will be <pod
	// name>-<resource name>, where <resource name> is the
	// PodResourceClaim.Name. Pod validation will reject the pod if the
	// concatenated name is not valid for a ResourceClaim (e.g. too long).
	// An existing ResourceClaim with that name that is not owned by the
	// pod will not be used for the pod to avoid using an unrelated
	// resource by mistake. Scheduling and pod startup are then blocked
	// until the unrelated ResourceClaim is removed.
	// This field is immutable and no changes will be made to the
	// corresponding ResourceClaim by the control plane after creating the
	// ResourceClaim.
	ResourceClaimTemplateName string `json:"resourceClaimTemplateName,omitempty"`
}

func (in *ClaimSource) DeepCopyInto(out *ClaimSource) {
	*out = *in
}

func (in *ClaimSource) DeepCopy() *ClaimSource {
	if in == nil {
		return nil
	}
	out := new(ClaimSource)
	in.DeepCopyInto(out)
	return out
}

type ContainerState struct {
	// Details about a waiting container
	Waiting *ContainerStateWaiting `json:"waiting,omitempty"`
	// Details about a running container
	Running *ContainerStateRunning `json:"running,omitempty"`
	// Details about a terminated container
	Terminated *ContainerStateTerminated `json:"terminated,omitempty"`
}

func (in *ContainerState) DeepCopyInto(out *ContainerState) {
	*out = *in
	if in.Waiting != nil {
		in, out := &in.Waiting, &out.Waiting
		*out = new(ContainerStateWaiting)
		(*in).DeepCopyInto(*out)
	}
	if in.Running != nil {
		in, out := &in.Running, &out.Running
		*out = new(ContainerStateRunning)
		(*in).DeepCopyInto(*out)
	}
	if in.Terminated != nil {
		in, out := &in.Terminated, &out.Terminated
		*out = new(ContainerStateTerminated)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ContainerState) DeepCopy() *ContainerState {
	if in == nil {
		return nil
	}
	out := new(ContainerState)
	in.DeepCopyInto(out)
	return out
}

type ScopedResourceSelectorRequirement struct {
	// The name of the scope that the selector applies to.
	ScopeName ResourceQuotaScope `json:"scopeName"`
	// Represents a scope's relationship to a set of values.
	// Valid operators are In, NotIn, Exists, DoesNotExist.
	Operator ScopeSelectorOperator `json:"operator"`
	// An array of string values. If the operator is In or NotIn,
	// the values array must be non-empty. If the operator is Exists or DoesNotExist,
	// the values array must be empty.
	// This array is replaced during a strategic merge patch.
	Values []string `json:"values"`
}

func (in *ScopedResourceSelectorRequirement) DeepCopyInto(out *ScopedResourceSelectorRequirement) {
	*out = *in
	if in.Values != nil {
		t := make([]string, len(in.Values))
		copy(t, in.Values)
		out.Values = t
	}
}

func (in *ScopedResourceSelectorRequirement) DeepCopy() *ScopedResourceSelectorRequirement {
	if in == nil {
		return nil
	}
	out := new(ScopedResourceSelectorRequirement)
	in.DeepCopyInto(out)
	return out
}

type ClientIPConfig struct {
	// timeoutSeconds specifies the seconds of ClientIP type session sticky time.
	// The value must be >0 && <=86400(for 1 day) if ServiceAffinity == "ClientIP".
	// Default value is 10800(for 3 hours).
	TimeoutSeconds int `json:"timeoutSeconds,omitempty"`
}

func (in *ClientIPConfig) DeepCopyInto(out *ClientIPConfig) {
	*out = *in
}

func (in *ClientIPConfig) DeepCopy() *ClientIPConfig {
	if in == nil {
		return nil
	}
	out := new(ClientIPConfig)
	in.DeepCopyInto(out)
	return out
}

type LoadBalancerIngress struct {
	// IP is set for load-balancer ingress points that are IP based
	// (typically GCE or OpenStack load-balancers)
	IP string `json:"ip,omitempty"`
	// Hostname is set for load-balancer ingress points that are DNS based
	// (typically AWS load-balancers)
	Hostname string `json:"hostname,omitempty"`
	// Ports is a list of records of service ports
	// If used, every port defined in the service should have an entry in it
	Ports []PortStatus `json:"ports"`
}

func (in *LoadBalancerIngress) DeepCopyInto(out *LoadBalancerIngress) {
	*out = *in
	if in.Ports != nil {
		l := make([]PortStatus, len(in.Ports))
		for i := range in.Ports {
			in.Ports[i].DeepCopyInto(&l[i])
		}
		out.Ports = l
	}
}

func (in *LoadBalancerIngress) DeepCopy() *LoadBalancerIngress {
	if in == nil {
		return nil
	}
	out := new(LoadBalancerIngress)
	in.DeepCopyInto(out)
	return out
}

type SecretReference struct {
	// name is unique within a namespace to reference a secret resource.
	Name string `json:"name,omitempty"`
	// namespace defines the space within which the secret name must be unique.
	Namespace string `json:"namespace,omitempty"`
}

func (in *SecretReference) DeepCopyInto(out *SecretReference) {
	*out = *in
}

func (in *SecretReference) DeepCopy() *SecretReference {
	if in == nil {
		return nil
	}
	out := new(SecretReference)
	in.DeepCopyInto(out)
	return out
}

type NodeSelectorTerm struct {
	// A list of node selector requirements by node's labels.
	MatchExpressions []NodeSelectorRequirement `json:"matchExpressions"`
	// A list of node selector requirements by node's fields.
	MatchFields []NodeSelectorRequirement `json:"matchFields"`
}

func (in *NodeSelectorTerm) DeepCopyInto(out *NodeSelectorTerm) {
	*out = *in
	if in.MatchExpressions != nil {
		l := make([]NodeSelectorRequirement, len(in.MatchExpressions))
		for i := range in.MatchExpressions {
			in.MatchExpressions[i].DeepCopyInto(&l[i])
		}
		out.MatchExpressions = l
	}
	if in.MatchFields != nil {
		l := make([]NodeSelectorRequirement, len(in.MatchFields))
		for i := range in.MatchFields {
			in.MatchFields[i].DeepCopyInto(&l[i])
		}
		out.MatchFields = l
	}
}

func (in *NodeSelectorTerm) DeepCopy() *NodeSelectorTerm {
	if in == nil {
		return nil
	}
	out := new(NodeSelectorTerm)
	in.DeepCopyInto(out)
	return out
}

type EmptyDirVolumeSource struct {
	// medium represents what type of storage medium should back this directory.
	// The default is "" which means to use the node's default medium.
	// Must be an empty string (default) or Memory.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
	Medium StorageMedium `json:"medium,omitempty"`
	// sizeLimit is the total amount of local storage required for this EmptyDir volume.
	// The size limit is also applicable for memory medium.
	// The maximum usage on memory medium EmptyDir would be the minimum value between
	// the SizeLimit specified here and the sum of memory limits of all containers in a pod.
	// The default is nil which means that the limit is undefined.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#emptydir
	SizeLimit *apiresource.Quantity `json:"sizeLimit,omitempty"`
}

func (in *EmptyDirVolumeSource) DeepCopyInto(out *EmptyDirVolumeSource) {
	*out = *in
	if in.SizeLimit != nil {
		in, out := &in.SizeLimit, &out.SizeLimit
		*out = new(apiresource.Quantity)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EmptyDirVolumeSource) DeepCopy() *EmptyDirVolumeSource {
	if in == nil {
		return nil
	}
	out := new(EmptyDirVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type GitRepoVolumeSource struct {
	// repository is the URL
	Repository string `json:"repository"`
	// revision is the commit hash for the specified revision.
	Revision string `json:"revision,omitempty"`
	// directory is the target directory name.
	// Must not contain or start with '..'.  If '.' is supplied, the volume directory will be the
	// git repository.  Otherwise, if specified, the volume will contain the git repository in
	// the subdirectory with the given name.
	Directory string `json:"directory,omitempty"`
}

func (in *GitRepoVolumeSource) DeepCopyInto(out *GitRepoVolumeSource) {
	*out = *in
}

func (in *GitRepoVolumeSource) DeepCopy() *GitRepoVolumeSource {
	if in == nil {
		return nil
	}
	out := new(GitRepoVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type SecretVolumeSource struct {
	// secretName is the name of the secret in the pod's namespace to use.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#secret
	SecretName string `json:"secretName,omitempty"`
	// items If unspecified, each key-value pair in the Data field of the referenced
	// Secret will be projected into the volume as a file whose name is the
	// key and content is the value. If specified, the listed keys will be
	// projected into the specified paths, and unlisted keys will not be
	// present. If a key is specified which is not present in the Secret,
	// the volume setup will error unless it is marked optional. Paths must be
	// relative and may not contain the '..' path or start with '..'.
	Items []KeyToPath `json:"items"`
	// defaultMode is Optional: mode bits used to set permissions on created files by default.
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values
	// for mode bits. Defaults to 0644.
	// Directories within the path are not affected by this setting.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	DefaultMode int `json:"defaultMode,omitempty"`
	// optional field specify whether the Secret or its keys must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *SecretVolumeSource) DeepCopyInto(out *SecretVolumeSource) {
	*out = *in
	if in.Items != nil {
		l := make([]KeyToPath, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *SecretVolumeSource) DeepCopy() *SecretVolumeSource {
	if in == nil {
		return nil
	}
	out := new(SecretVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ISCSIVolumeSource struct {
	// targetPortal is iSCSI Target Portal. The Portal is either an IP or ip_addr:port if the port
	// is other than default (typically TCP ports 860 and 3260).
	TargetPortal string `json:"targetPortal"`
	// iqn is the target iSCSI Qualified Name.
	IQN string `json:"iqn"`
	// lun represents iSCSI Target Lun number.
	Lun int `json:"lun"`
	// iscsiInterface is the interface Name that uses an iSCSI transport.
	// Defaults to 'default' (tcp).
	ISCSIInterface string `json:"iscsiInterface,omitempty"`
	// fsType is the filesystem type of the volume that you want to mount.
	// Tip: Ensure that the filesystem type is supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#iscsi
	FSType string `json:"fsType,omitempty"`
	// readOnly here will force the ReadOnly setting in VolumeMounts.
	// Defaults to false.
	ReadOnly bool `json:"readOnly,omitempty"`
	// portals is the iSCSI Target Portal List. The portal is either an IP or ip_addr:port if the port
	// is other than default (typically TCP ports 860 and 3260).
	Portals []string `json:"portals"`
	// chapAuthDiscovery defines whether support iSCSI Discovery CHAP authentication
	DiscoveryCHAPAuth bool `json:"chapAuthDiscovery,omitempty"`
	// chapAuthSession defines whether support iSCSI Session CHAP authentication
	SessionCHAPAuth bool `json:"chapAuthSession,omitempty"`
	// secretRef is the CHAP Secret for iSCSI target and initiator authentication
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
	// initiatorName is the custom iSCSI Initiator Name.
	// If initiatorName is specified with iscsiInterface simultaneously, new iSCSI interface
	// <target portal>:<volume name> will be created for the connection.
	InitiatorName string `json:"initiatorName,omitempty"`
}

func (in *ISCSIVolumeSource) DeepCopyInto(out *ISCSIVolumeSource) {
	*out = *in
	if in.Portals != nil {
		t := make([]string, len(in.Portals))
		copy(t, in.Portals)
		out.Portals = t
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ISCSIVolumeSource) DeepCopy() *ISCSIVolumeSource {
	if in == nil {
		return nil
	}
	out := new(ISCSIVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type GlusterfsVolumeSource struct {
	// endpoints is the endpoint name that details Glusterfs topology.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	EndpointsName string `json:"endpoints"`
	// path is the Glusterfs volume path.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	Path string `json:"path"`
	// readOnly here will force the Glusterfs volume to be mounted with read-only permissions.
	// Defaults to false.
	// More info: https://examples.k8s.io/volumes/glusterfs/README.md#create-a-pod
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *GlusterfsVolumeSource) DeepCopyInto(out *GlusterfsVolumeSource) {
	*out = *in
}

func (in *GlusterfsVolumeSource) DeepCopy() *GlusterfsVolumeSource {
	if in == nil {
		return nil
	}
	out := new(GlusterfsVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeClaimVolumeSource struct {
	// claimName is the name of a PersistentVolumeClaim in the same namespace as the pod using this volume.
	// More info: https://kubernetes.io/docs/concepts/storage/persistent-volumes#persistentvolumeclaims
	ClaimName string `json:"claimName"`
	// readOnly Will force the ReadOnly setting in VolumeMounts.
	// Default false.
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *PersistentVolumeClaimVolumeSource) DeepCopyInto(out *PersistentVolumeClaimVolumeSource) {
	*out = *in
}

func (in *PersistentVolumeClaimVolumeSource) DeepCopy() *PersistentVolumeClaimVolumeSource {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type RBDVolumeSource struct {
	// monitors is a collection of Ceph monitors.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	CephMonitors []string `json:"monitors"`
	// image is the rados image name.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	RBDImage string `json:"image"`
	// fsType is the filesystem type of the volume that you want to mount.
	// Tip: Ensure that the filesystem type is supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://kubernetes.io/docs/concepts/storage/volumes#rbd
	FSType string `json:"fsType,omitempty"`
	// pool is the rados pool name.
	// Default is rbd.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	RBDPool string `json:"pool,omitempty"`
	// user is the rados user name.
	// Default is admin.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	RadosUser string `json:"user,omitempty"`
	// keyring is the path to key ring for RBDUser.
	// Default is /etc/ceph/keyring.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	Keyring string `json:"keyring,omitempty"`
	// secretRef is name of the authentication secret for RBDUser. If provided
	// overrides keyring.
	// Default is nil.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
	// readOnly here will force the ReadOnly setting in VolumeMounts.
	// Defaults to false.
	// More info: https://examples.k8s.io/volumes/rbd/README.md#how-to-use-it
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *RBDVolumeSource) DeepCopyInto(out *RBDVolumeSource) {
	*out = *in
	if in.CephMonitors != nil {
		t := make([]string, len(in.CephMonitors))
		copy(t, in.CephMonitors)
		out.CephMonitors = t
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *RBDVolumeSource) DeepCopy() *RBDVolumeSource {
	if in == nil {
		return nil
	}
	out := new(RBDVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type FlexVolumeSource struct {
	// driver is the name of the driver to use for this volume.
	Driver string `json:"driver"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". The default filesystem depends on FlexVolume script.
	FSType string `json:"fsType,omitempty"`
	// secretRef is Optional: secretRef is reference to the secret object containing
	// sensitive information to pass to the plugin scripts. This may be
	// empty if no secret object is specified. If the secret object
	// contains more than one secret, all secrets are passed to the plugin
	// scripts.
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
	// readOnly is Optional: defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// options is Optional: this field holds extra command options if any.
	Options map[string]string `json:"options,omitempty"`
}

func (in *FlexVolumeSource) DeepCopyInto(out *FlexVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
	if in.Options != nil {
		in, out := &in.Options, &out.Options
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
}

func (in *FlexVolumeSource) DeepCopy() *FlexVolumeSource {
	if in == nil {
		return nil
	}
	out := new(FlexVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type CinderVolumeSource struct {
	// volumeID used to identify the volume in cinder.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	VolumeID string `json:"volumeID"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Examples: "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	FSType string `json:"fsType,omitempty"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	// More info: https://examples.k8s.io/mysql-cinder-pd/README.md
	ReadOnly bool `json:"readOnly,omitempty"`
	// secretRef is optional: points to a secret object containing parameters used to connect
	// to OpenStack.
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
}

func (in *CinderVolumeSource) DeepCopyInto(out *CinderVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *CinderVolumeSource) DeepCopy() *CinderVolumeSource {
	if in == nil {
		return nil
	}
	out := new(CinderVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type CephFSVolumeSource struct {
	// monitors is Required: Monitors is a collection of Ceph monitors
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	Monitors []string `json:"monitors"`
	// path is Optional: Used as the mounted root, rather than the full Ceph tree, default is /
	Path string `json:"path,omitempty"`
	// user is optional: User is the rados user name, default is admin
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	User string `json:"user,omitempty"`
	// secretFile is Optional: SecretFile is the path to key ring for User, default is /etc/ceph/user.secret
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	SecretFile string `json:"secretFile,omitempty"`
	// secretRef is Optional: SecretRef is reference to the authentication secret for User, default is empty.
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
	// readOnly is Optional: Defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	// More info: https://examples.k8s.io/volumes/cephfs/README.md#how-to-use-it
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *CephFSVolumeSource) DeepCopyInto(out *CephFSVolumeSource) {
	*out = *in
	if in.Monitors != nil {
		t := make([]string, len(in.Monitors))
		copy(t, in.Monitors)
		out.Monitors = t
	}
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *CephFSVolumeSource) DeepCopy() *CephFSVolumeSource {
	if in == nil {
		return nil
	}
	out := new(CephFSVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type DownwardAPIVolumeSource struct {
	// Items is a list of downward API volume file
	Items []DownwardAPIVolumeFile `json:"items"`
	// Optional: mode bits to use on created files by default. Must be a
	// Optional: mode bits used to set permissions on created files by default.
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// Defaults to 0644.
	// Directories within the path are not affected by this setting.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	DefaultMode int `json:"defaultMode,omitempty"`
}

func (in *DownwardAPIVolumeSource) DeepCopyInto(out *DownwardAPIVolumeSource) {
	*out = *in
	if in.Items != nil {
		l := make([]DownwardAPIVolumeFile, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *DownwardAPIVolumeSource) DeepCopy() *DownwardAPIVolumeSource {
	if in == nil {
		return nil
	}
	out := new(DownwardAPIVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type AzureFileVolumeSource struct {
	// secretName is the  name of secret that contains Azure Storage Account Name and Key
	SecretName string `json:"secretName"`
	// shareName is the azure share Name
	ShareName string `json:"shareName"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *AzureFileVolumeSource) DeepCopyInto(out *AzureFileVolumeSource) {
	*out = *in
}

func (in *AzureFileVolumeSource) DeepCopy() *AzureFileVolumeSource {
	if in == nil {
		return nil
	}
	out := new(AzureFileVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ConfigMapVolumeSource struct {
	LocalObjectReference `json:",inline"`
	// items if unspecified, each key-value pair in the Data field of the referenced
	// ConfigMap will be projected into the volume as a file whose name is the
	// key and content is the value. If specified, the listed keys will be
	// projected into the specified paths, and unlisted keys will not be
	// present. If a key is specified which is not present in the ConfigMap,
	// the volume setup will error unless it is marked optional. Paths must be
	// relative and may not contain the '..' path or start with '..'.
	Items []KeyToPath `json:"items"`
	// defaultMode is optional: mode bits used to set permissions on created files by default.
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// Defaults to 0644.
	// Directories within the path are not affected by this setting.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	DefaultMode int `json:"defaultMode,omitempty"`
	// optional specify whether the ConfigMap or its keys must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *ConfigMapVolumeSource) DeepCopyInto(out *ConfigMapVolumeSource) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
	if in.Items != nil {
		l := make([]KeyToPath, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ConfigMapVolumeSource) DeepCopy() *ConfigMapVolumeSource {
	if in == nil {
		return nil
	}
	out := new(ConfigMapVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ProjectedVolumeSource struct {
	// sources is the list of volume projections
	Sources []VolumeProjection `json:"sources"`
	// defaultMode are the mode bits used to set permissions on created files by default.
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// Directories within the path are not affected by this setting.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	DefaultMode int `json:"defaultMode,omitempty"`
}

func (in *ProjectedVolumeSource) DeepCopyInto(out *ProjectedVolumeSource) {
	*out = *in
	if in.Sources != nil {
		l := make([]VolumeProjection, len(in.Sources))
		for i := range in.Sources {
			in.Sources[i].DeepCopyInto(&l[i])
		}
		out.Sources = l
	}
}

func (in *ProjectedVolumeSource) DeepCopy() *ProjectedVolumeSource {
	if in == nil {
		return nil
	}
	out := new(ProjectedVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ScaleIOVolumeSource struct {
	// gateway is the host address of the ScaleIO API Gateway.
	Gateway string `json:"gateway"`
	// system is the name of the storage system as configured in ScaleIO.
	System string `json:"system"`
	// secretRef references to the secret for ScaleIO user and other
	// sensitive information. If this is not provided, Login operation will fail.
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
	// sslEnabled Flag enable/disable SSL communication with Gateway, default false
	SSLEnabled bool `json:"sslEnabled,omitempty"`
	// protectionDomain is the name of the ScaleIO Protection Domain for the configured storage.
	ProtectionDomain string `json:"protectionDomain,omitempty"`
	// storagePool is the ScaleIO Storage Pool associated with the protection domain.
	StoragePool string `json:"storagePool,omitempty"`
	// storageMode indicates whether the storage for a volume should be ThickProvisioned or ThinProvisioned.
	// Default is ThinProvisioned.
	StorageMode string `json:"storageMode,omitempty"`
	// volumeName is the name of a volume already created in the ScaleIO system
	// that is associated with this volume source.
	VolumeName string `json:"volumeName,omitempty"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs".
	// Default is "xfs".
	FSType string `json:"fsType,omitempty"`
	// readOnly Defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
}

func (in *ScaleIOVolumeSource) DeepCopyInto(out *ScaleIOVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ScaleIOVolumeSource) DeepCopy() *ScaleIOVolumeSource {
	if in == nil {
		return nil
	}
	out := new(ScaleIOVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type StorageOSVolumeSource struct {
	// volumeName is the human-readable name of the StorageOS volume.  Volume
	// names are only unique within a namespace.
	VolumeName string `json:"volumeName,omitempty"`
	// volumeNamespace specifies the scope of the volume within StorageOS.  If no
	// namespace is specified then the Pod's namespace will be used.  This allows the
	// Kubernetes name scoping to be mirrored within StorageOS for tighter integration.
	// Set VolumeName to any name to override the default behaviour.
	// Set to "default" if you are not using namespaces within StorageOS.
	// Namespaces that do not pre-exist within StorageOS will be created.
	VolumeNamespace string `json:"volumeNamespace,omitempty"`
	// fsType is the filesystem type to mount.
	// Must be a filesystem type supported by the host operating system.
	// Ex. "ext4", "xfs", "ntfs". Implicitly inferred to be "ext4" if unspecified.
	FSType string `json:"fsType,omitempty"`
	// readOnly defaults to false (read/write). ReadOnly here will force
	// the ReadOnly setting in VolumeMounts.
	ReadOnly bool `json:"readOnly,omitempty"`
	// secretRef specifies the secret to use for obtaining the StorageOS API
	// credentials.  If not specified, default values will be attempted.
	SecretRef *LocalObjectReference `json:"secretRef,omitempty"`
}

func (in *StorageOSVolumeSource) DeepCopyInto(out *StorageOSVolumeSource) {
	*out = *in
	if in.SecretRef != nil {
		in, out := &in.SecretRef, &out.SecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *StorageOSVolumeSource) DeepCopy() *StorageOSVolumeSource {
	if in == nil {
		return nil
	}
	out := new(StorageOSVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type CSIVolumeSource struct {
	// driver is the name of the CSI driver that handles this volume.
	// Consult with your admin for the correct name as registered in the cluster.
	Driver string `json:"driver"`
	// readOnly specifies a read-only configuration for the volume.
	// Defaults to false (read/write).
	ReadOnly bool `json:"readOnly,omitempty"`
	// fsType to mount. Ex. "ext4", "xfs", "ntfs".
	// If not provided, the empty value is passed to the associated CSI driver
	// which will determine the default filesystem to apply.
	FSType string `json:"fsType,omitempty"`
	// volumeAttributes stores driver-specific properties that are passed to the CSI
	// driver. Consult your driver's documentation for supported values.
	VolumeAttributes map[string]string `json:"volumeAttributes,omitempty"`
	// nodePublishSecretRef is a reference to the secret object containing
	// sensitive information to pass to the CSI driver to complete the CSI
	// NodePublishVolume and NodeUnpublishVolume calls.
	// This field is optional, and  may be empty if no secret is required. If the
	// secret object contains more than one secret, all secret references are passed.
	NodePublishSecretRef *LocalObjectReference `json:"nodePublishSecretRef,omitempty"`
}

func (in *CSIVolumeSource) DeepCopyInto(out *CSIVolumeSource) {
	*out = *in
	if in.VolumeAttributes != nil {
		in, out := &in.VolumeAttributes, &out.VolumeAttributes
		*out = make(map[string]string, len(*in))
		for k, v := range *in {
			(*out)[k] = v
		}
	}
	if in.NodePublishSecretRef != nil {
		in, out := &in.NodePublishSecretRef, &out.NodePublishSecretRef
		*out = new(LocalObjectReference)
		(*in).DeepCopyInto(*out)
	}
}

func (in *CSIVolumeSource) DeepCopy() *CSIVolumeSource {
	if in == nil {
		return nil
	}
	out := new(CSIVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type EphemeralVolumeSource struct {
	// Will be used to create a stand-alone PVC to provision the volume.
	// The pod in which this EphemeralVolumeSource is embedded will be the
	// owner of the PVC, i.e. the PVC will be deleted together with the
	// pod.  The name of the PVC will be `<pod name>-<volume name>` where
	// `<volume name>` is the name from the `PodSpec.Volumes` array
	// entry. Pod validation will reject the pod if the concatenated name
	// is not valid for a PVC (for example, too long).
	// An existing PVC with that name that is not owned by the pod
	// will *not* be used for the pod to avoid using an unrelated
	// volume by mistake. Starting the pod is then blocked until
	// the unrelated PVC is removed. If such a pre-created PVC is
	// meant to be used by the pod, the PVC has to updated with an
	// owner reference to the pod once the pod exists. Normally
	// this should not be necessary, but it may be useful when
	// manually reconstructing a broken cluster.
	// This field is read-only and no changes will be made by Kubernetes
	// to the PVC after it has been created.
	// Required, must not be nil.
	VolumeClaimTemplate *PersistentVolumeClaimTemplate `json:"volumeClaimTemplate,omitempty"`
}

func (in *EphemeralVolumeSource) DeepCopyInto(out *EphemeralVolumeSource) {
	*out = *in
	if in.VolumeClaimTemplate != nil {
		in, out := &in.VolumeClaimTemplate, &out.VolumeClaimTemplate
		*out = new(PersistentVolumeClaimTemplate)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EphemeralVolumeSource) DeepCopy() *EphemeralVolumeSource {
	if in == nil {
		return nil
	}
	out := new(EphemeralVolumeSource)
	in.DeepCopyInto(out)
	return out
}

type ConfigMapEnvSource struct {
	// The ConfigMap to select from.
	LocalObjectReference `json:",inline"`
	// Specify whether the ConfigMap must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *ConfigMapEnvSource) DeepCopyInto(out *ConfigMapEnvSource) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
}

func (in *ConfigMapEnvSource) DeepCopy() *ConfigMapEnvSource {
	if in == nil {
		return nil
	}
	out := new(ConfigMapEnvSource)
	in.DeepCopyInto(out)
	return out
}

type SecretEnvSource struct {
	// The Secret to select from.
	LocalObjectReference `json:",inline"`
	// Specify whether the Secret must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *SecretEnvSource) DeepCopyInto(out *SecretEnvSource) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
}

func (in *SecretEnvSource) DeepCopy() *SecretEnvSource {
	if in == nil {
		return nil
	}
	out := new(SecretEnvSource)
	in.DeepCopyInto(out)
	return out
}

type EnvVarSource struct {
	// Selects a field of the pod: supports metadata.name, metadata.namespace, `metadata.labels['<KEY>']`, `metadata.annotations['<KEY>']`,
	// spec.nodeName, spec.serviceAccountName, status.hostIP, status.podIP, status.podIPs.
	FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
	// Selects a resource of the container: only resources limits and requests
	// (limits.cpu, limits.memory, limits.ephemeral-storage, requests.cpu, requests.memory and requests.ephemeral-storage) are currently supported.
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	// Selects a key of a ConfigMap.
	ConfigMapKeyRef *ConfigMapKeySelector `json:"configMapKeyRef,omitempty"`
	// Selects a key of a secret in the pod's namespace
	SecretKeyRef *SecretKeySelector `json:"secretKeyRef,omitempty"`
}

func (in *EnvVarSource) DeepCopyInto(out *EnvVarSource) {
	*out = *in
	if in.FieldRef != nil {
		in, out := &in.FieldRef, &out.FieldRef
		*out = new(ObjectFieldSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.ResourceFieldRef != nil {
		in, out := &in.ResourceFieldRef, &out.ResourceFieldRef
		*out = new(ResourceFieldSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.ConfigMapKeyRef != nil {
		in, out := &in.ConfigMapKeyRef, &out.ConfigMapKeyRef
		*out = new(ConfigMapKeySelector)
		(*in).DeepCopyInto(*out)
	}
	if in.SecretKeyRef != nil {
		in, out := &in.SecretKeyRef, &out.SecretKeyRef
		*out = new(SecretKeySelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EnvVarSource) DeepCopy() *EnvVarSource {
	if in == nil {
		return nil
	}
	out := new(EnvVarSource)
	in.DeepCopyInto(out)
	return out
}

type ProbeHandler struct {
	// Exec specifies the action to take.
	Exec *ExecAction `json:"exec,omitempty"`
	// HTTPGet specifies the http request to perform.
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
	// TCPSocket specifies an action involving a TCP port.
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
	// GRPC specifies an action involving a GRPC port.
	GRPC *GRPCAction `json:"grpc,omitempty"`
}

func (in *ProbeHandler) DeepCopyInto(out *ProbeHandler) {
	*out = *in
	if in.Exec != nil {
		in, out := &in.Exec, &out.Exec
		*out = new(ExecAction)
		(*in).DeepCopyInto(*out)
	}
	if in.HTTPGet != nil {
		in, out := &in.HTTPGet, &out.HTTPGet
		*out = new(HTTPGetAction)
		(*in).DeepCopyInto(*out)
	}
	if in.TCPSocket != nil {
		in, out := &in.TCPSocket, &out.TCPSocket
		*out = new(TCPSocketAction)
		(*in).DeepCopyInto(*out)
	}
	if in.GRPC != nil {
		in, out := &in.GRPC, &out.GRPC
		*out = new(GRPCAction)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ProbeHandler) DeepCopy() *ProbeHandler {
	if in == nil {
		return nil
	}
	out := new(ProbeHandler)
	in.DeepCopyInto(out)
	return out
}

type LifecycleHandler struct {
	// Exec specifies the action to take.
	Exec *ExecAction `json:"exec,omitempty"`
	// HTTPGet specifies the http request to perform.
	HTTPGet *HTTPGetAction `json:"httpGet,omitempty"`
	// Deprecated. TCPSocket is NOT supported as a LifecycleHandler and kept
	// for the backward compatibility. There are no validation of this field and
	// lifecycle hooks will fail in runtime when tcp handler is specified.
	TCPSocket *TCPSocketAction `json:"tcpSocket,omitempty"`
}

func (in *LifecycleHandler) DeepCopyInto(out *LifecycleHandler) {
	*out = *in
	if in.Exec != nil {
		in, out := &in.Exec, &out.Exec
		*out = new(ExecAction)
		(*in).DeepCopyInto(*out)
	}
	if in.HTTPGet != nil {
		in, out := &in.HTTPGet, &out.HTTPGet
		*out = new(HTTPGetAction)
		(*in).DeepCopyInto(*out)
	}
	if in.TCPSocket != nil {
		in, out := &in.TCPSocket, &out.TCPSocket
		*out = new(TCPSocketAction)
		(*in).DeepCopyInto(*out)
	}
}

func (in *LifecycleHandler) DeepCopy() *LifecycleHandler {
	if in == nil {
		return nil
	}
	out := new(LifecycleHandler)
	in.DeepCopyInto(out)
	return out
}

type Capabilities struct {
	// Added capabilities
	Add []string `json:"add"`
	// Removed capabilities
	Drop []string `json:"drop"`
}

func (in *Capabilities) DeepCopyInto(out *Capabilities) {
	*out = *in
	if in.Add != nil {
		t := make([]string, len(in.Add))
		copy(t, in.Add)
		out.Add = t
	}
	if in.Drop != nil {
		t := make([]string, len(in.Drop))
		copy(t, in.Drop)
		out.Drop = t
	}
}

func (in *Capabilities) DeepCopy() *Capabilities {
	if in == nil {
		return nil
	}
	out := new(Capabilities)
	in.DeepCopyInto(out)
	return out
}

type PreferredSchedulingTerm struct {
	// Weight associated with matching the corresponding nodeSelectorTerm, in the range 1-100.
	Weight int `json:"weight"`
	// A node selector term, associated with the corresponding weight.
	Preference NodeSelectorTerm `json:"preference"`
}

func (in *PreferredSchedulingTerm) DeepCopyInto(out *PreferredSchedulingTerm) {
	*out = *in
	in.Preference.DeepCopyInto(&out.Preference)
}

func (in *PreferredSchedulingTerm) DeepCopy() *PreferredSchedulingTerm {
	if in == nil {
		return nil
	}
	out := new(PreferredSchedulingTerm)
	in.DeepCopyInto(out)
	return out
}

type PodAffinityTerm struct {
	// A label query over a set of resources, in this case pods.
	LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`
	// namespaces specifies a static list of namespace names that the term applies to.
	// The term is applied to the union of the namespaces listed in this field
	// and the ones selected by namespaceSelector.
	// null or empty namespaces list and null namespaceSelector means "this pod's namespace".
	Namespaces []string `json:"namespaces"`
	// This pod should be co-located (affinity) or not co-located (anti-affinity) with the pods matching
	// the labelSelector in the specified namespaces, where co-located is defined as running on a node
	// whose value of the label with key topologyKey matches that of any node on which any of the
	// selected pods is running.
	// Empty topologyKey is not allowed.
	TopologyKey string `json:"topologyKey"`
	// A label query over the set of namespaces that the term applies to.
	// The term is applied to the union of the namespaces selected by this field
	// and the ones listed in the namespaces field.
	// null selector and null or empty namespaces list means "this pod's namespace".
	// An empty selector ({}) matches all namespaces.
	NamespaceSelector *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
}

func (in *PodAffinityTerm) DeepCopyInto(out *PodAffinityTerm) {
	*out = *in
	if in.LabelSelector != nil {
		in, out := &in.LabelSelector, &out.LabelSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.Namespaces != nil {
		t := make([]string, len(in.Namespaces))
		copy(t, in.Namespaces)
		out.Namespaces = t
	}
	if in.NamespaceSelector != nil {
		in, out := &in.NamespaceSelector, &out.NamespaceSelector
		*out = new(metav1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PodAffinityTerm) DeepCopy() *PodAffinityTerm {
	if in == nil {
		return nil
	}
	out := new(PodAffinityTerm)
	in.DeepCopyInto(out)
	return out
}

type WeightedPodAffinityTerm struct {
	// weight associated with matching the corresponding podAffinityTerm,
	// in the range 1-100.
	Weight int `json:"weight"`
	// Required. A pod affinity term, associated with the corresponding weight.
	PodAffinityTerm PodAffinityTerm `json:"podAffinityTerm"`
}

func (in *WeightedPodAffinityTerm) DeepCopyInto(out *WeightedPodAffinityTerm) {
	*out = *in
	in.PodAffinityTerm.DeepCopyInto(&out.PodAffinityTerm)
}

func (in *WeightedPodAffinityTerm) DeepCopy() *WeightedPodAffinityTerm {
	if in == nil {
		return nil
	}
	out := new(WeightedPodAffinityTerm)
	in.DeepCopyInto(out)
	return out
}

type ContainerStateWaiting struct {
	// (brief) reason the container is not yet running.
	Reason string `json:"reason,omitempty"`
	// Message regarding why the container is not yet running.
	Message string `json:"message,omitempty"`
}

func (in *ContainerStateWaiting) DeepCopyInto(out *ContainerStateWaiting) {
	*out = *in
}

func (in *ContainerStateWaiting) DeepCopy() *ContainerStateWaiting {
	if in == nil {
		return nil
	}
	out := new(ContainerStateWaiting)
	in.DeepCopyInto(out)
	return out
}

type ContainerStateRunning struct {
	// Time at which the container was last (re-)started
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
}

func (in *ContainerStateRunning) DeepCopyInto(out *ContainerStateRunning) {
	*out = *in
	if in.StartedAt != nil {
		in, out := &in.StartedAt, &out.StartedAt
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ContainerStateRunning) DeepCopy() *ContainerStateRunning {
	if in == nil {
		return nil
	}
	out := new(ContainerStateRunning)
	in.DeepCopyInto(out)
	return out
}

type ContainerStateTerminated struct {
	// Exit status from the last termination of the container
	ExitCode int `json:"exitCode"`
	// Signal from the last termination of the container
	Signal int `json:"signal,omitempty"`
	// (brief) reason from the last termination of the container
	Reason string `json:"reason,omitempty"`
	// Message regarding the last termination of the container
	Message string `json:"message,omitempty"`
	// Time at which previous execution of the container started
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// Time at which the container last terminated
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
	// Container's ID in the format '<type>://<container_id>'
	ContainerID string `json:"containerID,omitempty"`
}

func (in *ContainerStateTerminated) DeepCopyInto(out *ContainerStateTerminated) {
	*out = *in
	if in.StartedAt != nil {
		in, out := &in.StartedAt, &out.StartedAt
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
	if in.FinishedAt != nil {
		in, out := &in.FinishedAt, &out.FinishedAt
		*out = new(metav1.Time)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ContainerStateTerminated) DeepCopy() *ContainerStateTerminated {
	if in == nil {
		return nil
	}
	out := new(ContainerStateTerminated)
	in.DeepCopyInto(out)
	return out
}

type PortStatus struct {
	// Port is the port number of the service port of which status is recorded here
	Port int `json:"port"`
	// Protocol is the protocol of the service port of which status is recorded here
	// The supported values are: "TCP", "UDP", "SCTP"
	Protocol Protocol `json:"protocol"`
	// Error is to record the problem with the service port
	// The format of the error shall comply with the following rules:
	// - built-in error values shall be specified in this file and those shall use
	// CamelCase names
	// - cloud provider specific error values must have names that comply with the
	// format foo.example.com/CamelCase.
	// ---
	// The regex it matches is (dns1123SubdomainFmt/)?(qualifiedNameFmt)
	Error string `json:"error,omitempty"`
}

func (in *PortStatus) DeepCopyInto(out *PortStatus) {
	*out = *in
}

func (in *PortStatus) DeepCopy() *PortStatus {
	if in == nil {
		return nil
	}
	out := new(PortStatus)
	in.DeepCopyInto(out)
	return out
}

type NodeSelectorRequirement struct {
	// The label key that the selector applies to.
	Key string `json:"key"`
	// Represents a key's relationship to a set of values.
	// Valid operators are In, NotIn, Exists, DoesNotExist. Gt, and Lt.
	Operator NodeSelectorOperator `json:"operator"`
	// An array of string values. If the operator is In or NotIn,
	// the values array must be non-empty. If the operator is Exists or DoesNotExist,
	// the values array must be empty. If the operator is Gt or Lt, the values
	// array must have a single element, which will be interpreted as an integer.
	// This array is replaced during a strategic merge patch.
	Values []string `json:"values"`
}

func (in *NodeSelectorRequirement) DeepCopyInto(out *NodeSelectorRequirement) {
	*out = *in
	if in.Values != nil {
		t := make([]string, len(in.Values))
		copy(t, in.Values)
		out.Values = t
	}
}

func (in *NodeSelectorRequirement) DeepCopy() *NodeSelectorRequirement {
	if in == nil {
		return nil
	}
	out := new(NodeSelectorRequirement)
	in.DeepCopyInto(out)
	return out
}

type KeyToPath struct {
	// key is the key to project.
	Key string `json:"key"`
	// path is the relative path of the file to map the key to.
	// May not be an absolute path.
	// May not contain the path element '..'.
	// May not start with the string '..'.
	Path string `json:"path"`
	// mode is Optional: mode bits used to set permissions on this file.
	// Must be an octal value between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// If not specified, the volume defaultMode will be used.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	Mode int `json:"mode,omitempty"`
}

func (in *KeyToPath) DeepCopyInto(out *KeyToPath) {
	*out = *in
}

func (in *KeyToPath) DeepCopy() *KeyToPath {
	if in == nil {
		return nil
	}
	out := new(KeyToPath)
	in.DeepCopyInto(out)
	return out
}

type DownwardAPIVolumeFile struct {
	// Required: Path is  the relative path name of the file to be created. Must not be absolute or contain the '..' path. Must be utf-8 encoded. The first item of the relative path must not start with '..'
	Path string `json:"path"`
	// Required: Selects a field of the pod: only annotations, labels, name and namespace are supported.
	FieldRef *ObjectFieldSelector `json:"fieldRef,omitempty"`
	// Selects a resource of the container: only resources limits and requests
	// (limits.cpu, limits.memory, requests.cpu and requests.memory) are currently supported.
	ResourceFieldRef *ResourceFieldSelector `json:"resourceFieldRef,omitempty"`
	// Optional: mode bits used to set permissions on this file, must be an octal value
	// between 0000 and 0777 or a decimal value between 0 and 511.
	// YAML accepts both octal and decimal values, JSON requires decimal values for mode bits.
	// If not specified, the volume defaultMode will be used.
	// This might be in conflict with other options that affect the file
	// mode, like fsGroup, and the result can be other mode bits set.
	Mode int `json:"mode,omitempty"`
}

func (in *DownwardAPIVolumeFile) DeepCopyInto(out *DownwardAPIVolumeFile) {
	*out = *in
	if in.FieldRef != nil {
		in, out := &in.FieldRef, &out.FieldRef
		*out = new(ObjectFieldSelector)
		(*in).DeepCopyInto(*out)
	}
	if in.ResourceFieldRef != nil {
		in, out := &in.ResourceFieldRef, &out.ResourceFieldRef
		*out = new(ResourceFieldSelector)
		(*in).DeepCopyInto(*out)
	}
}

func (in *DownwardAPIVolumeFile) DeepCopy() *DownwardAPIVolumeFile {
	if in == nil {
		return nil
	}
	out := new(DownwardAPIVolumeFile)
	in.DeepCopyInto(out)
	return out
}

type VolumeProjection struct {
	// secret information about the secret data to project
	Secret *SecretProjection `json:"secret,omitempty"`
	// downwardAPI information about the downwardAPI data to project
	DownwardAPI *DownwardAPIProjection `json:"downwardAPI,omitempty"`
	// configMap information about the configMap data to project
	ConfigMap *ConfigMapProjection `json:"configMap,omitempty"`
	// serviceAccountToken is information about the serviceAccountToken data to project
	ServiceAccountToken *ServiceAccountTokenProjection `json:"serviceAccountToken,omitempty"`
}

func (in *VolumeProjection) DeepCopyInto(out *VolumeProjection) {
	*out = *in
	if in.Secret != nil {
		in, out := &in.Secret, &out.Secret
		*out = new(SecretProjection)
		(*in).DeepCopyInto(*out)
	}
	if in.DownwardAPI != nil {
		in, out := &in.DownwardAPI, &out.DownwardAPI
		*out = new(DownwardAPIProjection)
		copy(**out, **in)
	}
	if in.ConfigMap != nil {
		in, out := &in.ConfigMap, &out.ConfigMap
		*out = new(ConfigMapProjection)
		(*in).DeepCopyInto(*out)
	}
	if in.ServiceAccountToken != nil {
		in, out := &in.ServiceAccountToken, &out.ServiceAccountToken
		*out = new(ServiceAccountTokenProjection)
		(*in).DeepCopyInto(*out)
	}
}

func (in *VolumeProjection) DeepCopy() *VolumeProjection {
	if in == nil {
		return nil
	}
	out := new(VolumeProjection)
	in.DeepCopyInto(out)
	return out
}

type PersistentVolumeClaimTemplate struct {
	// May contain labels and annotations that will be copied into the PVC
	// when creating it. No other fields are allowed and will be rejected during
	// validation.
	ObjectMeta *metav1.ObjectMeta `json:"metadata,omitempty"`
	// The specification for the PersistentVolumeClaim. The entire content is
	// copied unchanged into the PVC that gets created from this
	// template. The same fields as in a PersistentVolumeClaim
	// are also valid here.
	Spec PersistentVolumeClaimSpec `json:"spec"`
}

func (in *PersistentVolumeClaimTemplate) DeepCopyInto(out *PersistentVolumeClaimTemplate) {
	*out = *in
	if in.ObjectMeta != nil {
		in, out := &in.ObjectMeta, &out.ObjectMeta
		*out = new(metav1.ObjectMeta)
		(*in).DeepCopyInto(*out)
	}
	in.Spec.DeepCopyInto(&out.Spec)
}

func (in *PersistentVolumeClaimTemplate) DeepCopy() *PersistentVolumeClaimTemplate {
	if in == nil {
		return nil
	}
	out := new(PersistentVolumeClaimTemplate)
	in.DeepCopyInto(out)
	return out
}

type ObjectFieldSelector struct {
	// Version of the schema the FieldPath is written in terms of, defaults to "v1".
	APIVersion string `json:"apiVersion,omitempty"`
	// Path of the field to select in the specified API version.
	FieldPath string `json:"fieldPath"`
}

func (in *ObjectFieldSelector) DeepCopyInto(out *ObjectFieldSelector) {
	*out = *in
}

func (in *ObjectFieldSelector) DeepCopy() *ObjectFieldSelector {
	if in == nil {
		return nil
	}
	out := new(ObjectFieldSelector)
	in.DeepCopyInto(out)
	return out
}

type ResourceFieldSelector struct {
	// Container name: required for volumes, optional for env vars
	ContainerName string `json:"containerName,omitempty"`
	// Required: resource to select
	Resource string `json:"resource"`
	// Specifies the output format of the exposed resources, defaults to "1"
	Divisor *apiresource.Quantity `json:"divisor,omitempty"`
}

func (in *ResourceFieldSelector) DeepCopyInto(out *ResourceFieldSelector) {
	*out = *in
	if in.Divisor != nil {
		in, out := &in.Divisor, &out.Divisor
		*out = new(apiresource.Quantity)
		(*in).DeepCopyInto(*out)
	}
}

func (in *ResourceFieldSelector) DeepCopy() *ResourceFieldSelector {
	if in == nil {
		return nil
	}
	out := new(ResourceFieldSelector)
	in.DeepCopyInto(out)
	return out
}

type ConfigMapKeySelector struct {
	// The ConfigMap to select from.
	LocalObjectReference `json:",inline"`
	// The key to select.
	Key string `json:"key"`
	// Specify whether the ConfigMap or its key must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *ConfigMapKeySelector) DeepCopyInto(out *ConfigMapKeySelector) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
}

func (in *ConfigMapKeySelector) DeepCopy() *ConfigMapKeySelector {
	if in == nil {
		return nil
	}
	out := new(ConfigMapKeySelector)
	in.DeepCopyInto(out)
	return out
}

type SecretKeySelector struct {
	// The name of the secret in the pod's namespace to select from.
	LocalObjectReference `json:",inline"`
	// The key of the secret to select from.  Must be a valid secret key.
	Key string `json:"key"`
	// Specify whether the Secret or its key must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *SecretKeySelector) DeepCopyInto(out *SecretKeySelector) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
}

func (in *SecretKeySelector) DeepCopy() *SecretKeySelector {
	if in == nil {
		return nil
	}
	out := new(SecretKeySelector)
	in.DeepCopyInto(out)
	return out
}

type ExecAction struct {
	// Command is the command line to execute inside the container, the working directory for the
	// command  is root ('/') in the container's filesystem. The command is simply exec'd, it is
	// not run inside a shell, so traditional shell instructions ('|', etc) won't work. To use
	// a shell, you need to explicitly call out to that shell.
	// Exit status of 0 is treated as live/healthy and non-zero is unhealthy.
	Command []string `json:"command"`
}

func (in *ExecAction) DeepCopyInto(out *ExecAction) {
	*out = *in
	if in.Command != nil {
		t := make([]string, len(in.Command))
		copy(t, in.Command)
		out.Command = t
	}
}

func (in *ExecAction) DeepCopy() *ExecAction {
	if in == nil {
		return nil
	}
	out := new(ExecAction)
	in.DeepCopyInto(out)
	return out
}

type HTTPGetAction struct {
	// Path to access on the HTTP server.
	Path string `json:"path,omitempty"`
	// Name or number of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	Port utilintstr.IntOrString `json:"port"`
	// Host name to connect to, defaults to the pod IP. You probably want to set
	// "Host" in httpHeaders instead.
	Host string `json:"host,omitempty"`
	// Scheme to use for connecting to the host.
	// Defaults to HTTP.
	Scheme URIScheme `json:"scheme,omitempty"`
	// Custom headers to set in the request. HTTP allows repeated headers.
	HTTPHeaders []HTTPHeader `json:"httpHeaders"`
}

func (in *HTTPGetAction) DeepCopyInto(out *HTTPGetAction) {
	*out = *in
	in = out
	if in.HTTPHeaders != nil {
		l := make([]HTTPHeader, len(in.HTTPHeaders))
		for i := range in.HTTPHeaders {
			in.HTTPHeaders[i].DeepCopyInto(&l[i])
		}
		out.HTTPHeaders = l
	}
}

func (in *HTTPGetAction) DeepCopy() *HTTPGetAction {
	if in == nil {
		return nil
	}
	out := new(HTTPGetAction)
	in.DeepCopyInto(out)
	return out
}

type TCPSocketAction struct {
	// Number or name of the port to access on the container.
	// Number must be in the range 1 to 65535.
	// Name must be an IANA_SVC_NAME.
	Port utilintstr.IntOrString `json:"port"`
	// Optional: Host name to connect to, defaults to the pod IP.
	Host string `json:"host,omitempty"`
}

func (in *TCPSocketAction) DeepCopyInto(out *TCPSocketAction) {
	*out = *in
	in = out
}

func (in *TCPSocketAction) DeepCopy() *TCPSocketAction {
	if in == nil {
		return nil
	}
	out := new(TCPSocketAction)
	in.DeepCopyInto(out)
	return out
}

type GRPCAction struct {
	// Port number of the gRPC service. Number must be in the range 1 to 65535.
	Port int `json:"port"`
	// Service is the name of the service to place in the gRPC HealthCheckRequest
	// (see https://github.com/grpc/grpc/blob/master/doc/health-checking.md).
	// If this is not specified, the default behavior is defined by gRPC.
	Service string `json:"service,omitempty"`
}

func (in *GRPCAction) DeepCopyInto(out *GRPCAction) {
	*out = *in
}

func (in *GRPCAction) DeepCopy() *GRPCAction {
	if in == nil {
		return nil
	}
	out := new(GRPCAction)
	in.DeepCopyInto(out)
	return out
}

type SecretProjection struct {
	LocalObjectReference `json:",inline"`
	// items if unspecified, each key-value pair in the Data field of the referenced
	// Secret will be projected into the volume as a file whose name is the
	// key and content is the value. If specified, the listed keys will be
	// projected into the specified paths, and unlisted keys will not be
	// present. If a key is specified which is not present in the Secret,
	// the volume setup will error unless it is marked optional. Paths must be
	// relative and may not contain the '..' path or start with '..'.
	Items []KeyToPath `json:"items"`
	// optional field specify whether the Secret or its key must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *SecretProjection) DeepCopyInto(out *SecretProjection) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
	if in.Items != nil {
		l := make([]KeyToPath, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *SecretProjection) DeepCopy() *SecretProjection {
	if in == nil {
		return nil
	}
	out := new(SecretProjection)
	in.DeepCopyInto(out)
	return out
}

type DownwardAPIProjection []DownwardAPIVolumeFile

type ConfigMapProjection struct {
	LocalObjectReference `json:",inline"`
	// items if unspecified, each key-value pair in the Data field of the referenced
	// ConfigMap will be projected into the volume as a file whose name is the
	// key and content is the value. If specified, the listed keys will be
	// projected into the specified paths, and unlisted keys will not be
	// present. If a key is specified which is not present in the ConfigMap,
	// the volume setup will error unless it is marked optional. Paths must be
	// relative and may not contain the '..' path or start with '..'.
	Items []KeyToPath `json:"items"`
	// optional specify whether the ConfigMap or its keys must be defined
	Optional bool `json:"optional,omitempty"`
}

func (in *ConfigMapProjection) DeepCopyInto(out *ConfigMapProjection) {
	*out = *in
	out.LocalObjectReference = in.LocalObjectReference
	if in.Items != nil {
		l := make([]KeyToPath, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *ConfigMapProjection) DeepCopy() *ConfigMapProjection {
	if in == nil {
		return nil
	}
	out := new(ConfigMapProjection)
	in.DeepCopyInto(out)
	return out
}

type ServiceAccountTokenProjection struct {
	// audience is the intended audience of the token. A recipient of a token
	// must identify itself with an identifier specified in the audience of the
	// token, and otherwise should reject the token. The audience defaults to the
	// identifier of the apiserver.
	Audience string `json:"audience,omitempty"`
	// expirationSeconds is the requested duration of validity of the service
	// account token. As the token approaches expiration, the kubelet volume
	// plugin will proactively rotate the service account token. The kubelet will
	// start trying to rotate the token if the token is older than 80 percent of
	// its time to live or if the token is older than 24 hours.Defaults to 1 hour
	// and must be at least 10 minutes.
	ExpirationSeconds int64 `json:"expirationSeconds,omitempty"`
	// path is the path relative to the mount point of the file to project the
	// token into.
	Path string `json:"path"`
}

func (in *ServiceAccountTokenProjection) DeepCopyInto(out *ServiceAccountTokenProjection) {
	*out = *in
}

func (in *ServiceAccountTokenProjection) DeepCopy() *ServiceAccountTokenProjection {
	if in == nil {
		return nil
	}
	out := new(ServiceAccountTokenProjection)
	in.DeepCopyInto(out)
	return out
}

type HTTPHeader struct {
	// The header field name
	Name string `json:"name"`
	// The header field value
	Value string `json:"value"`
}

func (in *HTTPHeader) DeepCopyInto(out *HTTPHeader) {
	*out = *in
}

func (in *HTTPHeader) DeepCopy() *HTTPHeader {
	if in == nil {
		return nil
	}
	out := new(HTTPHeader)
	in.DeepCopyInto(out)
	return out
}
