/*
Copyright 2024 The Godel Scheduler Authors.

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

package api

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
)

type ReservationPlaceholderMap map[string]*v1.Pod

func (r ReservationPlaceholderMap) String() string {
	phString := make([]string, 0, len(r))
	for key := range r {
		phString = append(phString, key)
	}
	return strings.Join(phString, ", ")
}

type ReservationPlaceholdersOfNodes map[string]ReservationPlaceholderMap

func (p ReservationPlaceholdersOfNodes) String() string {
	nodeString := make([]string, 0, len(p))
	for nodeName, placeholders := range p {
		nodeString = append(nodeString, fmt.Sprintf("%s: %s", nodeName, placeholders.String()))
	}
	return strings.Join(nodeString, "; ")
}
