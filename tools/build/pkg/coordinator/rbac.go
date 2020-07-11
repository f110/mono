package coordinator

// +kubebuilder:rbac:groups=*,resources=pods;jobs;services,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=*,resources=pods/log,verbs=get
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create

// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
