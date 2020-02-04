
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


//go:generate go run ../../vendor/k8s.io/kube-openapi/cmd/openapi-gen/openapi-gen.go -o . --output-package ../../pkg/openapi --report-filename violations.report -i ../../pkg/apis/...,../../vendor/k8s.io/api/core/v1,../../vendor/k8s.io/apimachinery/pkg/apis/meta/v1 -h ../../boilerplate.go.txt
package openapi

