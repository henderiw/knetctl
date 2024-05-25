/*
Copyright 2024 Nokia.

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

package release

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	//docs "github.com/pkgserver-dev/pkgserver/internal/docs/generated/initdocs"

	"github.com/henderiw/knetctl/api/release"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	//"gopkg.in/yaml.v3"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"sigs.k8s.io/yaml"
)

// NewRunner returns a command runner.
func NewRunner(ctx context.Context, version string, cfg *genericclioptions.ConfigFlags) *Runner {
	r := &Runner{}
	cmd := &cobra.Command{
		Use:  "release RELEASEFILE DIR [flags]",
		Args: cobra.ExactArgs(2),
		//Short:   docs.InitShort,
		//Long:    docs.InitShort + "\n" + docs.InitLong,
		//Example: docs.InitExamples,
		PreRunE: r.preRunE,
		RunE:    r.runE,
	}

	r.Command = cmd
	r.cfg = cfg

	return r
}

func NewCommand(ctx context.Context, version string, kubeflags *genericclioptions.ConfigFlags) *cobra.Command {
	return NewRunner(ctx, version, kubeflags).Command
}

type Runner struct {
	Command *cobra.Command
	cfg     *genericclioptions.ConfigFlags
}

func (r *Runner) preRunE(_ *cobra.Command, _ []string) error {
	return nil
}

func (r *Runner) runE(c *cobra.Command, args []string) error {
	//ctx := c.Context()
	//log := log.FromContext(ctx)
	//log.Info("create packagerevision", "src", args[0], "dst", args[1])

	fmt.Println("file;", args[0])

	dir := args[1]
	os.MkdirAll(dir, 0755|os.ModeDir)

	b, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}

	rel := &release.Release{}
	if err := yaml.Unmarshal(b, rel); err != nil {
		return err
	}

	for _, app := range rel.Apps {
		path := filepath.Join(dir, app.Name+".yaml")
		url := app.URL
		if app.Version != nil {
			url = replaceVersionInURL(app.URL, *app.Version)
		}
		fmt.Printf("Downloading %s from %s\n", app.Name, url)
		err := downloadFile(url, path)
		if err != nil {
			return err
		}
		fmt.Printf("Downloaded %s successfully!\n", app.Name)
		if app.Image != nil && *app.Image != "" && app.Version != nil && *app.Version != "" {
			newImage := strings.ReplaceAll(*app.Image, "latest", *app.Version)

			err = replaceImageInFile(path, newImage)
			if err != nil {
				fmt.Printf("Error replacing image %s in file:", err)
			} else {
				fmt.Printf("Replaced image %s in %s successfully!\n", newImage, app.Name)
			}
		}
	}

	return nil
}

func replaceVersionInURL(originalURL, version string) string {
	parsedURL, err := url.Parse(originalURL)
	if err != nil {
		return originalURL
	}

	// Split the path by "/"
	pathParts := strings.Split(parsedURL.Path, "/")
	if len(pathParts) >= 4 {
		pathParts[3] = version // Replace the 4th element with the version
	}

	// Join the path parts back together
	parsedURL.Path = strings.Join(pathParts, "/")

	return parsedURL.String()
}

func downloadFile(url, filePath string) error {
	// Create a file to write the downloaded content
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Send HTTP GET request to the URL
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check if the response status code is OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file, status code: %d", resp.StatusCode)
	}

	// Copy the response body to the file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func replaceImageInFile(filePath, newImage string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	var updatedDocuments []string
	yamlDocuments, err := SplitDocuments(strings.ReplaceAll(string(data), "\r\n", "\n"))
	if err != nil {
		return err
	}
	for i := range yamlDocuments {
		// the Split used above will eat the tail '\n' from each resource. This may affect the
		// literal string value since '\n' is meaningful in it.
		if i != len(yamlDocuments)-1 {
			yamlDocuments[i] += "\n"
		}

		var artifact map[string]any
		err := yaml.Unmarshal([]byte(yamlDocuments[i]), &artifact)
		if err != nil {
			fmt.Printf("YAML ERROR1\n%s\n", yamlDocuments[i])
			return err
		}

		// Recursively replace the image field
		replaceImageField(artifact, newImage)

		updatedDoc, err := yaml.Marshal(&artifact)
		if err != nil {
			fmt.Printf("YAML ERROR\n")
			return err
		}

		updatedDocuments = append(updatedDocuments, string(updatedDoc))

	}

	updatedData := strings.Join(updatedDocuments, "---\n")
	err = os.WriteFile(filePath, []byte(updatedData), 0644)
	if err != nil {
		return err
	}

	return nil
}

func replaceImageField(data any, newImage string) {
	switch v := data.(type) {
	case map[any]any:
		for key, value := range v {
			if key == "image" {
				v[key] = newImage
			} else {
				replaceImageField(value, newImage)
			}
		}
	case map[string]interface{}:
		for key, value := range v {
			if key == "image" {
				v[key] = newImage
			} else {
				replaceImageField(value, newImage)
			}
		}
	case []interface{}:
		for _, item := range v {
			replaceImageField(item, newImage)
		}
	}
}

// splitDocuments returns a slice of all documents contained in a YAML string. Multiple documents can be divided by the
// YAML document separator (---). It allows for white space and comments to be after the separator on the same line,
// but will return an error if anything else is on the line.
func SplitDocuments(s string) ([]string, error) {
	docs := make([]string, 0)
	if len(s) > 0 {
		// The YAML document separator is any line that starts with ---
		yamlSeparatorRegexp := regexp.MustCompile(`\n---.*\n`)

		// Find all separators, check them for invalid content, and append each document to docs
		separatorLocations := yamlSeparatorRegexp.FindAllStringIndex(s, -1)
		prev := 0
		for i := range separatorLocations {
			loc := separatorLocations[i]
			separator := s[loc[0]:loc[1]]

			// If the next non-whitespace character on the line following the separator is not a comment, return an error
			trimmedContentAfterSeparator := strings.TrimSpace(separator[4:])
			if len(trimmedContentAfterSeparator) > 0 && trimmedContentAfterSeparator[0] != '#' {
				return nil, errors.Errorf("invalid document separator: %s", strings.TrimSpace(separator))
			}

			docs = append(docs, s[prev:loc[0]])
			prev = loc[1]
		}
		docs = append(docs, s[prev:])
	}

	return docs, nil
}
