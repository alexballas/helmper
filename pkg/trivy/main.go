package trivy

import (
	"context"
	"log/slog"
	"net/http"

	tcache "github.com/aquasecurity/trivy/pkg/cache"
	"github.com/aquasecurity/trivy/pkg/fanal/analyzer"
	"github.com/aquasecurity/trivy/pkg/fanal/artifact"
	image2 "github.com/aquasecurity/trivy/pkg/fanal/artifact/image"
	"github.com/aquasecurity/trivy/pkg/fanal/image"
	ftypes "github.com/aquasecurity/trivy/pkg/fanal/types"
	"github.com/aquasecurity/trivy/pkg/rpc/client"
	"github.com/aquasecurity/trivy/pkg/scanner"
	"github.com/aquasecurity/trivy/pkg/types"

	_ "modernc.org/sqlite" // sqlite driver for RPM DB and Java DB
)

type ScanOption struct {
	DockerHost    string
	TrivyServer   string
	Insecure      bool
	IgnoreUnfixed bool
}

func (opts ScanOption) Scan(reference string) (types.Report, error) {

	// initDB(insecureFlag)

	clientScanner := client.NewScanner(client.ScannerOption{
		RemoteURL: opts.TrivyServer,
		Insecure:  opts.Insecure,
	}, []client.Option(nil)...)

	typesImage, cleanup, err := image.NewContainerImage(context.TODO(), reference, ftypes.ImageOptions{
		RegistryOptions: ftypes.RegistryOptions{
			Insecure: opts.Insecure,
		},
		DockerOptions: ftypes.DockerOptions{
			Host: opts.DockerHost,
		},
		ImageSources: ftypes.AllImageSources,
	})
	if err != nil {
		slog.Error("NewContainerImage failed: %v", err)
		return types.Report{}, err
	}
	defer cleanup()
	remoteCache := tcache.NewRemoteCache(opts.TrivyServer, http.Header{}, opts.Insecure)
	cache := tcache.NopCache(remoteCache)
	artifactArtifact, err := image2.NewArtifact(typesImage, cache, artifact.Option{
		DisabledAnalyzers: []analyzer.Type{},
		DisabledHandlers:  nil,
		SkipFiles:         nil,
		SkipDirs:          nil,
		FilePatterns:      nil,
		NoProgress:        false,
		Insecure:          opts.Insecure,
		SBOMSources:       nil,
		RekorURL:          "https://rekor.sigstore.dev",
		// Parallel:          1,
		ImageOption: ftypes.ImageOptions{
			RegistryOptions: ftypes.RegistryOptions{
				Insecure: opts.Insecure,
			},
			DockerOptions: ftypes.DockerOptions{
				Host: opts.DockerHost,
			},
			ImageSources: ftypes.AllImageSources,
		},
	})
	if err != nil {
		slog.Error("NewArtifact failed: %v", err)
		return types.Report{}, err
	}

	scannerScanner := scanner.NewScanner(clientScanner, artifactArtifact)
	report, err := scannerScanner.ScanArtifact(context.TODO(), types.ScanOptions{
		VulnType:            []string{types.VulnTypeOS},
		Scanners:            types.AllScanners,
		ImageConfigScanners: types.AllImageConfigScanners,
		ScanRemovedPackages: false,
		ListAllPackages:     false,
		// LicenseCategories:   types.AllImageConfigScanners,
		FilePatterns:   nil,
		IncludeDevDeps: false,
	})
	if err != nil {
		slog.Error("ScanArtifact failed: %v", err, slog.StringValue(report.Metadata.OS.Family))
		return types.Report{}, err
	}

	if opts.IgnoreUnfixed {
		ignoreUnfixed(&report)
	}

	return report, nil

}

func ignoreUnfixed(report *types.Report) {

	// Homebrewed ignore unfixed
	for _, r := range report.Results {
		switch r.Class {
		case "ok-pkgs":
			vulns := []types.DetectedVulnerability{}
			for _, v := range r.Vulnerabilities {
				if v.FixedVersion != "" {
					// fixed
					vulns = append(vulns, v)
				}
			}

			count := len(r.Vulnerabilities) - len(vulns)
			if count == 0 {
				slog.Debug("removed unfixed vulnerabilities from result", slog.Int("count", count), slog.String("image", report.Metadata.ImageID))
			} else {
				slog.Info("removed unfixed vulnerabilities from result", slog.Int("count", count), slog.String("image", report.Metadata.ImageID))
			}

			r.Vulnerabilities = vulns
		}
	}
}
