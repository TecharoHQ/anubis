package multifile

import (
	"bytes"
	"embed"
	"errors"
	"io"
	"io/fs"
	"slices"
	"testing"

	datapkg "github.com/TecharoHQ/anubis/data"
	"github.com/goreleaser/fileglob"
	"k8s.io/apimachinery/pkg/util/yaml"
)

//go:embed testdata/*.yaml testdata/*.json
var testdata embed.FS

type data struct {
	Name string `json:"name" yaml:"name"`
}

type testCase struct {
	name          string
	fnames        []string
	wantYamlError bool
	validate      func(*testing.T, []data)
}

func runTestCase(tt testCase, t *testing.T, fsys fs.FS) {
	fin, err := YAMLList(fsys, tt.fnames)
	if err != nil {
		t.Fatal(err)
	}
	st, err := fin.Stat()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(st.Name())
	defer func() {
		if err := fin.Close(); err != nil {
			t.Logf("can't close file: %v", err)
		}
	}()

	var got []data

	dec := yaml.NewYAMLToJSONDecoder(fin)

outer:
	for {
		var thisGot []data
		err = dec.Decode(&thisGot)
		switch {
		case errors.Is(err, io.EOF):
			break outer
		case err == nil && tt.wantYamlError:
			t.Error("wanted yaml error but got none")
		case err != nil && !tt.wantYamlError:
			t.Errorf("did not want yaml error but got: %v", err)
		default:
			if err != nil {
				t.Log(err)
			}
		}
		got = append(got, thisGot...)
	}

	if tt.validate != nil {
		tt.validate(t, got)
	}
}

func TestYAMLList(t *testing.T) {
	for _, tt := range []testCase{
		{
			name:   "simple happy",
			fnames: []string{"testdata/a.yaml", "testdata/b.yaml"},
			validate: func(t *testing.T, got []data) {
				want := []data{
					{"foo"},
					{"bar"},
				}

				if !slices.EqualFunc(want, got, func(a data, b data) bool { return a.Name == b.Name }) {
					t.Logf("want: %#v", want)
					t.Logf(" got: %#v", got)
					t.Error("did not parse yaml correctly")
				}
			},
		},
		{
			name:          "invalid object when wanted list",
			fnames:        []string{"testdata/object.yaml"},
			wantYamlError: true,
			validate: func(t *testing.T, got []data) {
				if len(got) != 0 {
					t.Error("wanted data to not be populated")
				}
			},
		},
		{
			name:   "mixed indentation",
			fnames: []string{"testdata/a.yaml", "testdata/b.yaml", "testdata/indented.yaml"},
		},
		{
			name:   "mixed json and yaml",
			fnames: []string{"testdata/a.yaml", "testdata/data.json"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("embed fs", func(t *testing.T) {
				runTestCase(tt, t, testdata)
			})
			t.Run("os", func(t *testing.T) {
				runTestCase(tt, t, nil)
			})
		})
	}
}

func TestReadAllOfData(t *testing.T) {
	matches, err := fileglob.Glob("{apps,bots,bots/irc-bots,clients,common,crawlers,meta,services}/*.{yaml,yml}", fileglob.WithFs(datapkg.BotPolicies))
	if err != nil {
		t.Fatal(err)
	}
	t.Log(matches)

	fin, err := YAMLList(datapkg.BotPolicies, matches)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := fin.Close(); err != nil {
			t.Logf("can't close file: %v", err)
		}
	}()

	buf, err := io.ReadAll(fin)
	if err != nil {
		t.Fatal(err)
	}

	var got []any
	if err := yaml.NewYAMLToJSONDecoder(bytes.NewBuffer(buf)).Decode(&got); err != nil {
		t.Fatal(err)
	}

	t.Logf("read %d entries", len(got))
}
