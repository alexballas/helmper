package helm

import (
	"log"
	"log/slog"
	"strings"

	"github.com/ChristofferNissen/helmper/pkg/util/terminal"
	"golang.org/x/xerrors"
	"helm.sh/helm/v3/pkg/cli"
)

type ChartCollection struct {
	Charts []Chart `json:"charts"`
}

func (collection ChartCollection) pull() error {
	for _, chart := range collection.Charts {
		if _, err := chart.Pull(); err != nil {
			return err
		}
	}
	return nil
}

func (collection ChartCollection) addToHelmRepositoryConfig() error {
	for _, c := range collection.Charts {
		err := c.AddToHelmRepositoryFile()
		if err != nil {
			return err
		}
	}
	return nil
}

// configures helm and pulls charts to local fs
func (collection ChartCollection) SetupHelm(setters ...Option) (ChartCollection, error) {

	// Default Options
	args := &Options{
		Verbose: false,
		Update:  false,
	}

	for _, setter := range setters {
		setter(args)
	}

	for _, c := range collection.Charts {
		if !(strings.HasPrefix(c.Repo.URL, "http") || strings.HasPrefix(c.Repo.URL, "https")) {
			if strings.HasPrefix(c.Repo.URL, "oci") {
				return ChartCollection{}, xerrors.New("Helm only supports 'http and 'https' protocol for Helm Repositories. For oci protocol, see docs on the chart.oci configuration option in Helmper.")
			}
			return ChartCollection{}, xerrors.New("Helm only supports 'http and 'https' protocol for Helm Repositories")
		}
	}

	// Add Helm Repos
	err := collection.addToHelmRepositoryConfig()
	if err != nil {
		return ChartCollection{}, err
	}
	if args.Verbose {
		log.Printf("Added Helm repositories to config '%s' %s\n", cli.New().RepositoryConfig, terminal.GetCheckMarkEmoji())
	}

	// Update Helm Repos
	output, err := updateRepositories(args.Verbose, args.Update)
	if err != nil {
		return ChartCollection{}, err
	}
	// Log results
	if args.Verbose {
		log.Printf("Updated all Helm repositories %s\n%s", terminal.GetCheckMarkEmoji(), output)
	} else {
		log.Printf("Updated all Helm repositories %s\n", terminal.GetCheckMarkEmoji())
	}

	// Expand collection if semantic version range
	res := []Chart{}
	for _, c := range collection.Charts {

		vs, err := c.ResolveVersions()
		if err != nil {
			v, err := c.ResolveVersion()
			if err != nil {
				log.Fatal("version is not semver. skipping this version", slog.String("version", c.Version))
				continue
			}
			c.Version = v
			res = append(res, c)
		}

		for _, v := range vs {
			c := c
			c.Version = v
			res = append(res, c)
		}
	}
	collection.Charts = res

	// Pull Helm Charts
	err = collection.pull()
	if err != nil {
		return ChartCollection{}, err
	}
	if args.Verbose {
		log.Println("Pulled Helm Charts")
	}

	return collection, nil
}
