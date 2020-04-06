package v1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// UserTypeLocal represents a User who is native to K8S (e.g. backed by GKE).
	UserTypeLocal = "User"
	// UserTypeExternal represents a User who is managed externally (e.g. in GitHub) and will have a linked ServiceAccount.
	UserTypeExternal = "ServiceAccount"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:openapi-gen=true

// User represents a git user so we have a cache to find by email address
type User struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object's metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Deprecated, use Spec
	User UserDetails `json:"user,omitempty" protobuf:"bytes,2,opt,name=user"`

	Spec UserDetails `json:"spec,omitempty" protobuf:"bytes,3,opt,name=spec"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserList is a list of User resources
type UserList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []User `json:"items"`
}

// UserDetails containers details of a user
type UserDetails struct {
	Login             string             `json:"login,omitempty"  protobuf:"bytes,1,opt,name=login"`
	Name              string             `json:"name,omitempty"  protobuf:"bytes,2,opt,name=name"`
	Email             string             `json:"email,omitempty"  protobuf:"bytes,3,opt,name=email"`
	CreationTimestamp *metav1.Time       `json:"creationTimestamp,omitempty" protobuf:"bytes,4,opt,name=creationTimestamp"`
	URL               string             `json:"url,omitempty"  protobuf:"bytes,5,opt,name=url"`
	AvatarURL         string             `json:"avatarUrl,omitempty"  protobuf:"bytes,6,opt,name=avatarUrl"`
	ServiceAccount    string             `json:"serviceAccount,omitempty"  protobuf:"bytes,7,opt,name=serviceAccount"`
	Accounts          []AccountReference `json:"accountReference,omitempty"  protobuf:"bytes,8,opt,name=accountReference"`
	ExternalUser      bool               `json:"externalUser,omitempty"  protobuf:"bytes,9,opt,name=externalUser"`
}

// AccountReference is a reference to a user account in another system that is attached to this user
type AccountReference struct {
	Provider string `json:"provider,omitempty"  protobuf:"bytes,1,opt,name=provider"`
	ID       string `json:"id,omitempty"  protobuf:"bytes,2,opt,name=id"`
}

// SubjectKind returns the subject kind of user - either "User" (native K8S user) or "ServiceAccount" (externally managed
// user).
func (u *User) SubjectKind() string {
	if u.Spec.ExternalUser {
		return UserTypeExternal
	}
	return UserTypeLocal
}

// UnmarshalJSON method merges the deprecated User field and the Spec field on User, preferring Spec
func (u *User) UnmarshalJSON(bs []byte) error {
	var intermediate map[string]json.RawMessage
	err := json.Unmarshal(bs, &intermediate)
	if err != nil {
		return err
	}
	kind, ok := intermediate["kind"]
	if ok {
		err = json.Unmarshal(kind, &u.Kind)
		if err != nil {
			return err
		}
	}
	apiVersion, ok := intermediate["apiVersion"]
	if ok {
		err = json.Unmarshal(apiVersion, &u.APIVersion)
		if err != nil {
			return err
		}
	}
	objectMeta, ok := intermediate["metadata"]
	if ok {
		err = json.Unmarshal(objectMeta, &u.ObjectMeta)
		if err != nil {
			return err
		}
	}
	user, userOk := intermediate["user"]
	spec, specOk := intermediate["spec"]
	if specOk && userOk {
		// If both are there, merge them, with preference given to spec
		merged := UserDetails{}
		err = json.Unmarshal(user, &merged)
		if err != nil {
			return err
		}
		err = json.Unmarshal(spec, &merged)
		if err != nil {
			return err
		}
		u.Spec = merged
		u.User = merged
	} else if !specOk && userOk {
		// If only user is there, copy it into spec
		err = json.Unmarshal(user, &u.Spec)
		if err != nil {
			return err
		}
		err = json.Unmarshal(user, &u.User)
		if err != nil {
			return err
		}
	} else if specOk {
		// If only spec is there, copy it into user
		err = json.Unmarshal(spec, &u.User)
		if err != nil {
			return err
		}
		err = json.Unmarshal(spec, &u.Spec)
		if err != nil {
			return err
		}
	}
	return nil
}
