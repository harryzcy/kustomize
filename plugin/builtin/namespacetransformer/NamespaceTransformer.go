// Copyright 2019 The Kubernetes Authors.
// SPDX-License-Identifier: Apache-2.0

//go:generate pluginator
package main

import (
	"fmt"

	"sigs.k8s.io/kustomize/api/filters/namespace"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/errors"
	"sigs.k8s.io/yaml"
)

// Change or set the namespace of non-cluster level resources.
//
//nolint:tagalign
type plugin struct {
	types.ObjectMeta       `json:"metadata,omitempty" yaml:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	FieldSpecs             []types.FieldSpec                `json:"fieldSpecs,omitempty" yaml:"fieldSpecs,omitempty"`
	UnsetOnly              bool                             `json:"unsetOnly" yaml:"unsetOnly"`
	SetRoleBindingSubjects namespace.RoleBindingSubjectMode `json:"setRoleBindingSubjects" yaml:"setRoleBindingSubjects"`
}

var KustomizePlugin plugin //nolint:gochecknoglobals

func (p *plugin) Config(
	_ *resmap.PluginHelpers, c []byte) (err error) {
	p.Namespace = ""
	p.FieldSpecs = nil
	if err := yaml.Unmarshal(c, p); err != nil {
		return errors.WrapPrefixf(err, "unmarshalling NamespaceTransformer config")
	}
	switch p.SetRoleBindingSubjects {
	case namespace.AllServiceAccountSubjects, namespace.DefaultSubjectsOnly, namespace.NoSubjects:
		// valid
	case namespace.SubjectModeUnspecified:
		p.SetRoleBindingSubjects = namespace.DefaultSubjectsOnly
	default:
		return errors.Errorf("invalid value %q for setRoleBindingSubjects: "+
			"must be one of %q, %q or %q", p.SetRoleBindingSubjects,
			namespace.DefaultSubjectsOnly, namespace.NoSubjects, namespace.AllServiceAccountSubjects)
	}

	return nil
}

func (p *plugin) Transform(m resmap.ResMap) error {
	if len(p.Namespace) == 0 {
		return nil
	}
	for _, r := range m.Resources() {
		if r.IsNilOrEmpty() {
			// Don't mutate empty objects?
			continue
		}
		r.StorePreviousId()
		if err := r.ApplyFilter(namespace.Filter{
			Namespace:              p.Namespace,
			FsSlice:                p.FieldSpecs,
			SetRoleBindingSubjects: p.SetRoleBindingSubjects,
			UnsetOnly:              p.UnsetOnly,
		}); err != nil {
			return err
		}
		matches := m.GetMatchingResourcesByCurrentId(r.CurId().Equals)
		if len(matches) != 1 {
			return fmt.Errorf(
				"namespace transformation produces ID conflict: %+v", matches)
		}
	}
	return nil
}
