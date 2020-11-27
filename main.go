// go install github.com/noc-tech/xliff-merge

package main

import (
	"context"
	"encoding/xml"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"cloud.google.com/go/translate"
	log "github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"google.golang.org/api/option"
)

var (
	path                  = flag.String("path", "angular/src/locale", "Angular locale path")
	googleTranslate       = flag.Bool("googleTranslate", false, "Use Google Translate to translate new texts")
	apiKey                = flag.String("apikey", "", "Google Translate API key")
	googleTranslateClient *translate.Client
)

func main() {
	flag.Parse()

	if *googleTranslate && *apiKey == "" {
		log.Fatal("You must provide Google Translate API key")
	}

	if *googleTranslate {
		// Creates a google translate client.
		var err error
		googleTranslateClient, err = translate.NewClient(context.Background(), option.WithAPIKey(*apiKey))
		if err != nil {
			log.Fatalf("Failed to create client: %v", err)
		}
	}

	var locales []string

	files, err := ioutil.ReadDir(*path)
	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		name := f.Name()
		if strings.HasSuffix(name, ".xlf") {
			locales = append(locales, name[9:11])
		}
	}

	var english *Xliff
	english, err = readXML("en")
	if err != nil {
		log.Fatal(err)
	}
	english.TrgLang = "en"

	for _, lang := range locales {
		m := make(map[string]*Unit)
		trg, err := readXML(lang)
		if err != nil {
			log.Fatal(err)
		}
		//save and continue if source and target language is the same
		if lang == "en" {
			err = saveXML(lang, english)
			if err != nil {
				log.Errorf("error on saving xml file: %s\n", err)
			}
			continue
		}
		// Sets the target language.
		targetLang, err := language.Parse(lang)
		if err != nil {
			log.Fatalf("Failed to parse target language: %v", err)
		}

		// create map for old translations
		for _, unit := range trg.File.Units {
			m[unit.ID] = unit
		}
		trg.TrgLang = lang
		trg.File.Units = nil
		for _, unit := range english.File.Units {
			var target string
			var state *State
			if val, ok := m[unit.ID]; ok && val.Segment.Target != nil {
				target = val.Segment.Target.Text
				state = val.Segment.State
			} else {
				if *googleTranslate && !strings.Contains(unit.Segment.Source.Text, "{") {
					// Translates the text into target language.
					if translations, err := googleTranslateClient.Translate(context.Background(), []string{unit.Segment.Source.Text}, targetLang, nil); err == nil {
						target = translations[0].Text
						state = &State{Text: "not-checked"}
					}
				}
			}
			if target == "" {
				target = unit.Segment.Source.Text
				state = &State{Text: "initial"}
			}
			trg.File.Units = append(trg.File.Units, &Unit{
				ID: unit.ID,
				Segment: Segment{
					Source: unit.Segment.Source,
					Target: &Target{Text: target},
					State:  state,
				},
			})
		}
		err = saveXML(lang, trg)
		if err != nil {
			log.Errorf("error on saving xml file: %s", err)
		}
		log.Infof("%s saved.\n", lang)
	}
}

func readXML(lang string) (*Xliff, error) {
	xmlFile, err := os.Open(fmt.Sprintf("%s/messages.%s.xlf", *path, lang))
	if err != nil {
		return nil, err
	}
	defer xmlFile.Close()

	b, err := ioutil.ReadAll(xmlFile)
	if err != nil {
		return nil, err
	}
	var q Xliff
	xml.Unmarshal(b, &q)
	return &q, nil
}

func saveXML(lang string, content *Xliff) error {
	output, err := xml.MarshalIndent(content, "", "  ")
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(fmt.Sprintf("%s/messages.%s.xlf", *path, lang), []byte(xml.Header+string(output)), 0644)
	if err != nil {
		return err
	}
	return nil
}

type Xliff struct {
	XMLName struct{} `xml:"xliff"`
	Version string   `xml:"version,attr"`
	Xmlns   string   `xml:"xmlns,attr"`
	SrcLang string   `xml:"srcLang,attr"`
	TrgLang string   `xml:"trgLang,attr"`
	File    File     `xml:"file"`
}

type File struct {
	Original string  `xml:"original,attr"`
	ID       string  `xml:"id,attr"`
	Units    []*Unit `xml:"unit"`
}

type Unit struct {
	ID      string  `xml:"id,attr"`
	Segment Segment `xml:"segment"`
}

type Segment struct {
	Source Source  `xml:"source"`
	Target *Target `xml:"target,omitempty"`
	State  *State  `xml:"state,omitempty"`
}

type Source struct {
	Text string `xml:",innerxml"`
}

type Target struct {
	Text string `xml:",innerxml"`
}

type State struct {
	Text string `xml:",innerxml"`
}
