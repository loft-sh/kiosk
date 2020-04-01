/*
Copyright 2020 DevSpace Technologies Inc.

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

package constants

const (
	// UserPrefix is used to prefix users for the index
	UserPrefix = "user:"
	// GroupPrefix is used to prefix groups for the index
	GroupPrefix = "group:"
)

const (
	// IndexBySubjects indexes rolebindings / clusterrolebindings by their subjects
	IndexBySubjects = "bysubjects"
	// IndexByAccount indexes objects by their referenced account
	IndexByAccount = "byaccount"
	// IndexByTemplates indexes objects by their referenced template
	IndexByTemplate = "bytemplate"
)
