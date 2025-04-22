package xml_util

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

const XmlFilePath = "./xml/"

type Data struct {
	Document *Document `xml:"DOKUMENT"`
}

type Document struct {
	Items []Item `xml:"POZYCJE>POZYCJA"`
}

type Item struct {
	CatalogNumber string  `xml:"TOWAR>NUMER_KATALOGOWY"`
	Name          string  `xml:"TOWAR>NAZWA"`
	Ean           string  `xml:"TOWAR>EAN"`
	Amount        float64 `xml:"ILOSC"`

	Duplicate  bool `xml:"-"` // not in xml
	IsMainItem bool `xml:"-"` // not in xml
}

type CombinedData struct {
	XMLName xml.Name `xml:"CombinedDocument"`
	Items   []Item   `xml:"Items>Item"`
}

var (
	XmlItemsFromFilename = make(map[string][]Item)
	xmlMu                sync.Mutex
)

func DecodeXMLToStruct(filePath string) (*Data, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data Data
	decoder := xml.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data, nil
}

func DecodeAllXMLToStruct() (*CombinedData, []string, error) {
	xmlMu.Lock()
	defer xmlMu.Unlock()

	files, err := os.ReadDir(XmlFilePath)
	if err != nil {
		return nil, nil, err
	}

	combined := &CombinedData{}
	resFiles := make([]string, 0)
	eanToFirstItem := make(map[string]*Item)

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".xml" {
			continue
		}

		filePath := filepath.Join(XmlFilePath, f.Name())
		data, err := parseXMLFile(filePath)
		if err != nil {
			return nil, nil, err
		}

		if data != nil && data.Document != nil {
			for i := range data.Document.Items {
				item := &data.Document.Items[i]

				itemEan := item.Ean
				if itemEan != "" {
					_, err := strconv.Atoi(itemEan)
					if err != nil {
						return nil, nil, err
					}
				}

				if firstItem, exists := eanToFirstItem[itemEan]; exists {
					item.Duplicate = true
					firstItem.Duplicate = true
					firstItem.IsMainItem = true
				} else {
					eanToFirstItem[itemEan] = item
				}

				combined.Items = append(combined.Items, *item)
			}

			XmlItemsFromFilename[f.Name()] = data.Document.Items
			resFiles = append(resFiles, f.Name())
		}
	}

	return combined, resFiles, nil
}

func parseXMLFile(filePath string) (*Data, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, err
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var data Data
	decoder := xml.NewDecoder(file)
	if err := decoder.Decode(&data); err != nil {
		return nil, err
	}

	return &data, nil
}
