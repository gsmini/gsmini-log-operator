/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// GsminiLogSpec defines the desired state of GsminiLog
type GsminiLogSpec struct {
	LogDestination string `json:"log_destination,omitempty"` //日志目的地 oss|mysql|es
	LogDestUri     string `json:"log_dest_uri,omitempty"`    //链接地址 比如mysql://user@password:120.0.0.1/db_name
	LogReportType  string `json:"log_report_type,omitempty"` //报警类型 dingding|feishu|phone|sms
	LogReportUri   string `json:"log_report_uri,omitempty"`  //报警地址 比如:https://4da32r.feishu.com/xxxx 表示推送到非书这个地址
	LogRule        string `json:"log_rule,omitempty"`        //报警规则匹配，会去正则表达匹配
}

// GsminiLogStatus defines the observed state of GsminiLog
type GsminiLogStatus struct {
	LogNumber      int64            `json:"log_number,omitempty"`       //日志捕捉总条数
	LogRuleNumber  map[string]int64 `json:"log_rule_number,omitempty"`  //触发报警条数条数 {"dingding":12,"feishu":20}
	LogWriteNumber map[string]int64 `json:"log_write_number,omitempty"` //日志写入条数 {"oss":12,"es":20}

}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// GsminiLog is the Schema for the gsminilogs API
type GsminiLog struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GsminiLogSpec   `json:"spec,omitempty"`
	Status GsminiLogStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// GsminiLogList contains a list of GsminiLog
type GsminiLogList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GsminiLog `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GsminiLog{}, &GsminiLogList{})
}
