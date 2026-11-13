// SPDX-License-Identifier: Apache-2.0

package evidence

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/complytime/complyctl/pkg/provider"
)

// Reference creates an evidence/reference pair from one validated retained artifact.
// Prefixes identify artifact kind; Token supplies instance uniqueness independently
// of lossy logical-ID helpers. FileURL receives only the validated absolute locator.
func (evaluation *Evaluation) Reference(name, idPrefix, referencePrefix, kind, title, description, remarks string) (provider.Evidence, provider.MappingReference, error) {
	artifact := evaluation.validated[name]
	if artifact == nil || !filepath.IsAbs(artifact.Path) {
		return provider.Evidence{}, provider.MappingReference{}, fmt.Errorf("artifact %q has not been validated", name)
	}
	refID := referencePrefix + evaluation.Token
	item := provider.Evidence{
		ID: idPrefix + evaluation.Token, Type: kind,
		CollectedAt: evaluation.CollectionTime.Format(time.RFC3339),
		Source: &provider.EvidenceSource{ReferenceID: refID, Coordinate: artifact.Coordinate,
			Digest: artifact.Digest, Remarks: remarks},
	}
	ref := provider.MappingReference{ID: refID, Title: PublicContext(title),
		Description: PublicContext(description), URL: FileURL(artifact.Path)}
	return item, ref, nil
}
