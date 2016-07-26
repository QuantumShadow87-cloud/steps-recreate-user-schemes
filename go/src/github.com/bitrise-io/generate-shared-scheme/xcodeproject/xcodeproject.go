package xcodeproject

import (
	"errors"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bitrise-io/go-utils/errorutil"
	"github.com/bitrise-io/go-utils/fileutil"
	"github.com/bitrise-io/go-utils/pathutil"
)

func runCommand(envs []string, dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if len(envs) > 0 {
		cmd.Env = append(cmd.Env, envs...)
	}
	if dir != "" {
		cmd.Dir = dir
	}
	outBytes, err := cmd.CombinedOutput()
	outStr := string(outBytes)
	return strings.TrimSpace(outStr), err
}

func properReturn(err error, out string) error {
	if err == nil {
		return nil
	}

	if errorutil.IsExitStatusError(err) && out != "" {
		return errors.New(out)
	}
	return err
}

// SharedSchemeFiles ...
func SharedSchemeFiles(projectOrWorkspacePth string) ([]string, error) {
	pattern := filepath.Join(projectOrWorkspacePth, "xcshareddata", "xcschemes", "*.xcscheme")
	return filepath.Glob(pattern)
}

// SharedSchemes ...
func SharedSchemes(projectOrWorkspacePth string) ([]string, error) {
	schemeFiles, err := SharedSchemeFiles(projectOrWorkspacePth)
	if err != nil {
		return []string{}, err
	}

	regexp := regexp.MustCompile(filepath.Join(projectOrWorkspacePth, "xcshareddata", "xcschemes", "(?P<scheme>.+).xcscheme"))

	schemeMap := map[string]bool{}
	for _, schemeFile := range schemeFiles {
		match := regexp.FindStringSubmatch(schemeFile)
		if len(match) > 1 {
			schemeMap[match[1]] = true
		}
	}

	schemes := []string{}
	for scheme := range schemeMap {
		schemes = append(schemes, scheme)
	}

	return schemes, nil
}

// UserSchemeFiles ...
func UserSchemeFiles(projectOrWorkspacePth string) ([]string, error) {
	pattern := filepath.Join(projectOrWorkspacePth, "xcuserdata", "*.xcuserdatad", "xcschemes", "*.xcscheme")
	return filepath.Glob(pattern)
}

// UserSchemes ...
func UserSchemes(projectOrWorkspacePth string) ([]string, error) {
	schemeFiles, err := UserSchemeFiles(projectOrWorkspacePth)
	if err != nil {
		return []string{}, err
	}

	regexp := regexp.MustCompile(filepath.Join(projectOrWorkspacePth, "xcuserdata", ".*.xcuserdatad", "xcschemes", "(?P<scheme>.+).xcscheme"))

	schemes := []string{}
	for _, schemeFile := range schemeFiles {
		match := regexp.FindStringSubmatch(schemeFile)
		if len(match) > 1 {
			schemes = append(schemes, match[1])
		}
	}

	return schemes, nil
}

// ReCreateProjectUserSchemes ...
func ReCreateProjectUserSchemes(projectPth string) error {
	rubyScriptContent := `require 'xcodeproj'
require 'json'

project_path = ENV['project_path']

begin
  raise 'empty path' if project_path.empty?

  project = Xcodeproj::Project.open(project_path)
  project.recreate_user_schemes
  project.save
rescue => ex
  puts(ex.inspect.to_s)
  puts('--- Stack trace: ---')
  puts(ex.backtrace.to_s)
  exit(1)
end
`

	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise")
	if err != nil {
		return err
	}

	rubyScriptPth := path.Join(tmpDir, "recreate_user_schemes.rb")
	if err := fileutil.WriteStringToFile(rubyScriptPth, rubyScriptContent); err != nil {
		return err
	}

	projectDir := filepath.Dir(projectPth)
	projectBase := filepath.Base(projectPth)

	out, err := runCommand([]string{fmt.Sprintf("project_path=%s", projectBase), "LC_ALL=en_US.UTF-8"}, projectDir, "ruby", rubyScriptPth)
	return properReturn(err, out)
}

// WorkspaceProjectReferences ...
func WorkspaceProjectReferences(workspace string) ([]string, error) {
	projects := []string{}

	workspaceDir := filepath.Dir(workspace)

	xcworkspacedataPth := path.Join(workspace, "contents.xcworkspacedata")
	if exist, err := pathutil.IsPathExists(xcworkspacedataPth); err != nil {
		return []string{}, err
	} else if !exist {
		return []string{}, fmt.Errorf("contents.xcworkspacedata does not exist at: %s", xcworkspacedataPth)
	}

	xcworkspacedataStr, err := fileutil.ReadStringFromFile(xcworkspacedataPth)
	if err != nil {
		return []string{}, err
	}

	xcworkspacedataLines := strings.Split(xcworkspacedataStr, "\n")
	fileRefStart := false
	regexp := regexp.MustCompile(`location = "(.+):(.+).xcodeproj"`)

	for _, line := range xcworkspacedataLines {
		if strings.Contains(line, "<FileRef") {
			fileRefStart = true
			continue
		}

		if fileRefStart {
			fileRefStart = false
			matches := regexp.FindStringSubmatch(line)
			if len(matches) == 3 {
				projectName := matches[2]
				project := filepath.Join(workspaceDir, projectName+".xcodeproj")
				projects = append(projects, project)
			}
		}
	}

	return projects, nil
}

/*
// ShareUserScheme ...
func ShareUserScheme(projectPth, scheme string) error {
	rubyScriptContent := `require 'xcodeproj'
require 'json'

project_path = ENV['project_path']
scheme = ENV['scheme']

begin
  project = Xcodeproj::XCScheme.share_scheme(project_path, scheme)
rescue => ex
  puts(ex.inspect.to_s)
  puts('--- Stack trace: ---')
  puts(ex.backtrace.to_s)
  exit(1)
end`

	tmpDir, err := pathutil.NormalizedOSTempDirPath("bitrise")
	if err != nil {
		return err
	}

	rubyScriptPth := path.Join(tmpDir, "share_scheme.rb")
	if err := fileutil.WriteStringToFile(rubyScriptPth, rubyScriptContent); err != nil {
		return err
	}

	projectPthEnv := "project_path=" + projectPth
	schemeEnv := "scheme=" + scheme

	out, err := runCommand([]string{projectPthEnv, schemeEnv}, "ruby", rubyScriptPth)
	return properReturn(err, out)
}
*/