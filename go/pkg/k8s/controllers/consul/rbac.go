package consul

// +kubebuilder:rbac:groups=consul.f110.dev,resources=consulbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=consul.f110.dev,resources=consulbackups/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=*,resources=pods;secrets;services,verbs=get;list
// +kubebuilder:rbac:groups=*,resources=pods/portforward,verbs=get;list;create
