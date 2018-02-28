// Copyright (c) 2018 Tigera, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package calc

import (
	"strings"

	"github.com/projectcalico/felix/dispatcher"
	"github.com/projectcalico/felix/proto"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/k8s/conversion"
	"github.com/projectcalico/libcalico-go/lib/backend/model"

	log "github.com/sirupsen/logrus"
)

type kind int

const (
	KindUnknown kind = iota
	KindServiceAccount
)

// profileDecoder takes updates from a dispatcher, determines if the profile is a Kubernetes Service Account, and
// if it is, generates a dataplane update or remove for it.
type profileDecoder struct {
	callbacks passthruCallbacks
}

func NewProfileDecoder(callbacks passthruCallbacks) *profileDecoder {
	return &profileDecoder{callbacks}
}

func (p *profileDecoder) RegisterWith(d *dispatcher.Dispatcher) {
	d.Register(model.ProfileKey{}, p.OnUpdate)
}

func (p *profileDecoder) OnUpdate(update api.Update) (filterOut bool) {
	// This type assertion is safe because we only registered for Profile updates.
	key := update.Key.(model.ProfileKey)
	id, kind := classifyProfile(key)
	if kind == KindUnknown {
		// We only care about Profiles that are service accounts.
		return false
	}
	if update.Value == nil {
		p.callbacks.OnServiceAccountRemove(id)
	} else {
		profile := update.Value.(*model.Profile)
		msg := proto.ServiceAccountUpdate{Id: &id, Labels: decodeServiceAccountLabels(profile.Labels)}
		p.callbacks.OnServiceAccountUpdate(&msg)
	}
	return false
}

func classifyProfile(key model.ProfileKey) (proto.ServiceAccountID, kind) {
	if strings.HasPrefix(conversion.ServiceAccountProfileNamePrefix, key.Name) {
		c := strings.Split(key.Name, ".")
		if len(c) == 3 {
			return proto.ServiceAccountID{Name: c[2], Namespace: c[1]}, KindServiceAccount
		} else {
			log.WithField("name", key.Name).Warn("Profile with SA prefix could not be parsed.")
			return proto.ServiceAccountID{}, KindUnknown
		}
	} else {
		return proto.ServiceAccountID{}, KindUnknown
	}
}

func decodeServiceAccountLabels(in map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range in {
		k = strings.TrimPrefix(k, conversion.ServiceAccountLabelPrefix)
		out[k] = v
	}
	return out
}
