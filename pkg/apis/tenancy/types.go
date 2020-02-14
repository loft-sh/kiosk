package tenancy

import (
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// SpaceAnnotationAccount describes the account the space belongs to
	SpaceAnnotationAccount = "kiosk.sh/account"
	// SpaceAnnotationInitializing is used to describe a space as initializing and block role creation for this namespace
	SpaceAnnotationInitializing = "kiosk.sh/initializing"
)

// +genclient
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Account defines an account
type Account struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Spec   AccountSpec
	Status AccountStatus
}

// AccountSpec defines a single account configuration
type AccountSpec struct {
	// Space defines default options for created spaces by the account
	// +optional
	Space AccountSpace `json:"space,omitempty"`

	// Subjects are the account users
	// +optional
	Subjects []rbacv1.Subject `json:"subjects,omitempty"`
}

// AccountSpace defines properties how many spaces can be owned by the account and how they should be created
type AccountSpace struct {
	// This defines the cluster role that will be used for the rolebinding when
	// creating a new space for the selected subjects
	// +optional
	ClusterRole *string `json:"clusterRole,omitempty"`

	// Limit defines how many spaces are allowed to be owned by this account. If no value is specified,
	// unlimited spaces can be created by the account (if the users have the rights to create spaces)
	// +optional
	Limit *int `json:"limit,omitempty"`

	// TemplateInstances are templates that should be created by default in a newly created space by
	// this account. Kiosk makes sure that these templates are deployed successfully, before the users of
	// this account will get access to the space
	// +optional
	TemplateInstances []AccountTemplateInstanceTemplate `json:"templateInstances,omitempty"`

	// SpaceTemplate defines a space template with default annotations and labels the space should have after
	// creation
	// +optional
	SpaceTemplate AccountSpaceTemplate `json:"spaceTemplate,omitempty"`
}

// AccountSpaceTemplate defines a space template
type AccountSpaceTemplate struct {
	// The default metadata of the space to create
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

// AccountTemplateInstanceTemplate defines a template instance template
type AccountTemplateInstanceTemplate struct {
	// The metadata of the template instace to create
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// The spec of the template instance
	// +optional
	Spec TemplateInstanceSpec `json:"spec,omitempty"`
}

// TemplateInstanceSpec holds the expected cluster status of the template instance
type TemplateInstanceSpec struct {
	// The template to instantiate. This is an immutable field
	Template string `json:"template"`
}

// AccountStatus describes the current status of the account is the cluster
type AccountStatus struct {
	// +optional
	Namespaces []AccountNamespaceStatus `json:"namespaces,omitempty"`
}

// AccountNamespaceStatus is the status for the account access objects that belong to the account
type AccountNamespaceStatus struct {
	// +optional
	Name string `json:"name,omitempty"`
}

// +genclient
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Space struct {
	metav1.TypeMeta
	metav1.ObjectMeta
	Spec   SpaceSpec
	Status SpaceStatus
}

type SpaceSpec struct {
	// Account is the owning account of the space, this will be either filled automatically, if the requesting user is only part
	// of a single account or has to be filled by the user.
	// +optional
	Account string `json:"account,omitempty"`

	// Finalizers is an opaque list of values that must be empty to permanently remove object from storage.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/namespaces/
	// +optional
	Finalizers []corev1.FinalizerName `json:"finalizers,omitempty"`
}

type SpaceStatus struct {
	// Phase is the current lifecycle phase of the namespace.
	// More info: https://kubernetes.io/docs/tasks/administer-cluster/namespaces/
	// +optional
	Phase corev1.NamespacePhase `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type AccountList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Account
}

func (Account) NewStatus() interface{} {
	return AccountStatus{}
}

func (pc *Account) GetStatus() interface{} {
	return pc.Status
}

func (pc *Account) SetStatus(s interface{}) {
	pc.Status = s.(AccountStatus)
}

func (pc *Account) GetSpec() interface{} {
	return pc.Spec
}

func (pc *Account) SetSpec(s interface{}) {
	pc.Spec = s.(AccountSpec)
}

func (pc *Account) GetObjectMeta() *metav1.ObjectMeta {
	return &pc.ObjectMeta
}

func (pc *Account) SetGeneration(generation int64) {
	pc.ObjectMeta.Generation = generation
}

func (pc Account) GetGeneration() int64 {
	return pc.ObjectMeta.Generation
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SpaceList struct {
	metav1.TypeMeta
	metav1.ListMeta
	Items []Space
}

func (Space) NewStatus() interface{} {
	return SpaceStatus{}
}

func (pc *Space) GetStatus() interface{} {
	return pc.Status
}

func (pc *Space) SetStatus(s interface{}) {
	pc.Status = s.(SpaceStatus)
}

func (pc *Space) GetSpec() interface{} {
	return pc.Spec
}

func (pc *Space) SetSpec(s interface{}) {
	pc.Spec = s.(SpaceSpec)
}

func (pc *Space) GetObjectMeta() *metav1.ObjectMeta {
	return &pc.ObjectMeta
}

func (pc *Space) SetGeneration(generation int64) {
	pc.ObjectMeta.Generation = generation
}

func (pc Space) GetGeneration() int64 {
	return pc.ObjectMeta.Generation
}
