package controller

// BlockchainNode controller RBAC

// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes/finalizers,verbs=update
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services;configmaps;persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;patch;delete
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete

// NodeHealth controller RBAC

// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=nodes.chainplane.io,resources=blockchainnodes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch;create;delete;patch
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=endpoints,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch
