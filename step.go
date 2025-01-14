package main

import (
	"fmt"
	"path/filepath"

	"github.com/bitrise-io/go-steputils/v2/stepconf"
	"github.com/bitrise-io/go-utils/colorstring"
	"github.com/bitrise-io/go-utils/log"
	"github.com/bitrise-io/go-utils/pathutil"
	"github.com/bitrise-io/go-xcode/xcodeproject/xcscheme"
)

// Input ...
type Input struct {
	ProjectPath string `env:"project_path,file"`
}

type Config struct {
	ContainerPath string
}

type SchemeGenerator struct {
	inputParser stepconf.InputParser
}

func NewSchemeGenerator(inputParser stepconf.InputParser) SchemeGenerator {
	return SchemeGenerator{
		inputParser: inputParser,
	}
}

func (g SchemeGenerator) ProcessConfig() (Config, error) {
	var input Input
	err := g.inputParser.Parse(&input)
	if err != nil {
		return Config{}, err
	}
	stepconf.Print(input)

	containerPath, err := pathutil.AbsPath(input.ProjectPath)
	if err != nil {
		return Config{}, fmt.Errorf("failed to get absolute path for: %s: %w", input.ProjectPath, err)
	}

	return Config{
		ContainerPath: containerPath,
	}, nil
}

func (g SchemeGenerator) Run(cfg Config) error {
	container, err := openContainer(cfg.ContainerPath)
	if err != nil {
		return fmt.Errorf("opening container failed: %w", err)
	}

	fmt.Println()
	log.Infof("Collecting existing Schemes...")
	containerToSchemes, err := container.schemes()
	if err != nil {
		log.Warnf("Failed to list schemes: %s", err)
	}

	if len(containerToSchemes) > 0 {
		log.Printf("Schemes:")
		printSchemes(true, containerToSchemes, cfg.ContainerPath)

		preexistingSharedSchemes := numberOfSharedSchemes(containerToSchemes)
		if preexistingSharedSchemes > 0 {
			fmt.Println()
			log.Donef("There are %d shared Scheme(s).", preexistingSharedSchemes)
			return nil
		}
	}

	// Generate schemes
	fmt.Println()
	log.Warnf("No shared Schemes found...")
	log.Warnf("The newly generated Schemes may differ from the ones in your Project.")
	log.Warnf("Make sure to share your Schemes, to prevent unexpected behaviour.")

	fmt.Println()
	log.Infof("Generating Schemes...")

	projects, missingProjects, err := container.projects()
	if err != nil {
		return fmt.Errorf("getting projects failed: %w", err)
	}

	for _, missingProject := range missingProjects {
		log.Warnf("Skipping project (%s), as it is not present", pathRelativeToWorkspace(missingProject, cfg.ContainerPath))
	}

	for _, project := range projects {
		log.Printf("Recreating Schemes for: %s", filepath.Base(project.Path))
		schemes := project.ReCreateSchemes()

		for _, scheme := range schemes {
			if err := project.SaveSharedScheme(scheme); err != nil {
				return fmt.Errorf("saving scheme %s failed: %w", scheme.Name, err)
			}
		}
	}

	container, err = openContainer(cfg.ContainerPath)
	if err != nil {
		return fmt.Errorf("opening the updated container failed: %w", err)
	}
	containerToSchemesNew, err := container.schemes()
	if err != nil {
		return fmt.Errorf("getting new schemes failed: %w", err)
	}

	numberOfNewSchemes := numberOfSharedSchemes(containerToSchemesNew)

	if numberOfNewSchemes == 0 {
		fmt.Println()
		return fmt.Errorf("no schemes generated")
	}

	fmt.Println()
	log.Printf("Created Schemes:")
	printSchemes(false, containerToSchemesNew, cfg.ContainerPath)

	fmt.Println()
	log.Donef("Generated %d shared Scheme(s).", numberOfNewSchemes)

	return nil
}

func pathRelativeToWorkspace(project, workspace string) string {
	parentDir, _ := filepath.Split(workspace)
	relPath, err := filepath.Rel(filepath.Join(parentDir), project)
	if err != nil {
		log.Warnf("%s", err)
		return project
	}

	return relPath
}

func numberOfSharedSchemes(containerToSchemes map[string][]xcscheme.Scheme) int {
	var count int
	for _, schemes := range containerToSchemes {
		for _, scheme := range schemes {
			if scheme.IsShared {
				count++
			}
		}
	}

	return count
}

func printSchemes(includeUserSchemes bool, containerToSchemes map[string][]xcscheme.Scheme, containerPath string) {
	for container, schemes := range containerToSchemes {
		log.Printf("- %s", pathRelativeToWorkspace(container, containerPath))
		for _, scheme := range schemes {
			if scheme.IsShared {
				log.Printf("  - %s (Shared)", scheme.Name)
			} else if includeUserSchemes {
				log.Printf(colorstring.Yellow(fmt.Sprintf("  - %s (User)", scheme.Name)))
			}
		}
	}
}
