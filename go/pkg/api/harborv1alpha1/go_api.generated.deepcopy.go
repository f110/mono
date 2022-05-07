package harborv1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type HarborProject struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              HarborProjectSpec   `json:"spec"`
	Status            HarborProjectStatus `json:"status"`
}

func (in *HarborProject) DeepCopyInto(out *HarborProject) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *HarborProject) DeepCopy() *HarborProject {
	if in == nil {
		return nil
	}
	out := new(HarborProject)
	in.DeepCopyInto(out)
	return out
}

func (in *HarborProject) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type HarborProjectList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []HarborProject `json:"items"`
}

func (in *HarborProjectList) DeepCopyInto(out *HarborProjectList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]HarborProject, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *HarborProjectList) DeepCopy() *HarborProjectList {
	if in == nil {
		return nil
	}
	out := new(HarborProjectList)
	in.DeepCopyInto(out)
	return out
}

func (in *HarborProjectList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type HarborRobotAccount struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Spec              HarborRobotAccountSpec   `json:"spec"`
	Status            HarborRobotAccountStatus `json:"status"`
}

func (in *HarborRobotAccount) DeepCopyInto(out *HarborRobotAccount) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *HarborRobotAccount) DeepCopy() *HarborRobotAccount {
	if in == nil {
		return nil
	}
	out := new(HarborRobotAccount)
	in.DeepCopyInto(out)
	return out
}

func (in *HarborRobotAccount) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type HarborRobotAccountList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []HarborRobotAccount `json:"items"`
}

func (in *HarborRobotAccountList) DeepCopyInto(out *HarborRobotAccountList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		l := make([]HarborRobotAccount, len(in.Items))
		for i := range in.Items {
			in.Items[i].DeepCopyInto(&l[i])
		}
		out.Items = l
	}
}

func (in *HarborRobotAccountList) DeepCopy() *HarborRobotAccountList {
	if in == nil {
		return nil
	}
	out := new(HarborRobotAccountList)
	in.DeepCopyInto(out)
	return out
}

func (in *HarborRobotAccountList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

type HarborProjectSpec struct {
	// public is an access level of the project.
	//  If public sets true, then anyone can read.
	Public bool `json:"public"`
}

func (in *HarborProjectSpec) DeepCopyInto(out *HarborProjectSpec) {
	*out = *in
}

func (in *HarborProjectSpec) DeepCopy() *HarborProjectSpec {
	if in == nil {
		return nil
	}
	out := new(HarborProjectSpec)
	in.DeepCopyInto(out)
	return out
}

type HarborProjectStatus struct {
	Ready     bool   `json:"ready"`
	ProjectId int    `json:"projectId"`
	Registry  string `json:"registry"`
}

func (in *HarborProjectStatus) DeepCopyInto(out *HarborProjectStatus) {
	*out = *in
}

func (in *HarborProjectStatus) DeepCopy() *HarborProjectStatus {
	if in == nil {
		return nil
	}
	out := new(HarborProjectStatus)
	in.DeepCopyInto(out)
	return out
}

type HarborRobotAccountSpec struct {
	ProjectNamespace string `json:"projectNamespace"`
	ProjectName      string `json:"projectName"`
	// secret_name is a name of docker config secret.
	SecretName string `json:"secretName"`
}

func (in *HarborRobotAccountSpec) DeepCopyInto(out *HarborRobotAccountSpec) {
	*out = *in
}

func (in *HarborRobotAccountSpec) DeepCopy() *HarborRobotAccountSpec {
	if in == nil {
		return nil
	}
	out := new(HarborRobotAccountSpec)
	in.DeepCopyInto(out)
	return out
}

type HarborRobotAccountStatus struct {
	Ready   bool `json:"ready"`
	RobotId int  `json:"robotId"`
}

func (in *HarborRobotAccountStatus) DeepCopyInto(out *HarborRobotAccountStatus) {
	*out = *in
}

func (in *HarborRobotAccountStatus) DeepCopy() *HarborRobotAccountStatus {
	if in == nil {
		return nil
	}
	out := new(HarborRobotAccountStatus)
	in.DeepCopyInto(out)
	return out
}
