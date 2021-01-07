package grafana

// +kubebuilder:rbac:groups=grafana.f110.dev,resources=grafanas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=grafana.f110.dev,resources=grafanas/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=grafana.f110.dev,resources=grafanausers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=grafana.f110.dev,resources=grafanausers/status,verbs=get;update;patch

// +kubebuilder:rbac:groups=*,resources=secrets;services,verbs=get;list;watch;create;update;patch;delete
