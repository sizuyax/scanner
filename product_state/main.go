package product_state

import (
	"fmt"
	xutil "scanner/xml_util"
	"strings"
	"sync"
)

var (
	ErrDuplicateSameFile  = fmt.Errorf("[Duplicate in same file]")
	ErrDuplicateDiffFiles = fmt.Errorf("[Duplicate in different files]")
	ErrNoneEAN            = fmt.Errorf("[None EAN]")
)

type (
	ProductState struct {
		CatalogNumber string   `json:"catalog_number"`
		Amount        float64  `json:"amount"`
		Scanned       uint     `json:"scanned"`
		Difference    int      `json:"difference"`
		Status        uint8    `json:"status"`
		ErrorDesc     string   `json:"error_desc,omitempty"`
		SourceFiles   []string `json:"source_files,omitempty"`
	}

	AmountProductDupKey struct {
		Ean      string
		Filename string
	}
)

var (
	productState map[string]*ProductState
	eanToFiles   map[string]map[string]struct{} // EAN -> map[filename]struct{}
	stateMu      sync.RWMutex

	alreadyProcessedDuplicates map[string]struct{}
	amountProductsInDuplicate  map[AmountProductDupKey]float64
)

func init() {
	productState = make(map[string]*ProductState)
	eanToFiles = make(map[string]map[string]struct{})
	alreadyProcessedDuplicates = make(map[string]struct{})
	amountProductsInDuplicate = make(map[AmountProductDupKey]float64)
}

func SetProductState(ean string, ps *ProductState) {
	stateMu.Lock()
	defer stateMu.Unlock()
	productState[ean] = ps
}

func GetProductState(ean string) *ProductState {
	stateMu.RLock()
	defer stateMu.RUnlock()
	return productState[ean]
}

func GetAllProductStates() map[string]*ProductState {
	stateMu.RLock()
	defer stateMu.RUnlock()

	states := make(map[string]*ProductState)
	for k, v := range productState {
		states[k] = &ProductState{
			CatalogNumber: v.CatalogNumber,
			Amount:        v.Amount,
			Scanned:       v.Scanned,
			Difference:    v.Difference,
			Status:        v.Status,
			ErrorDesc:     v.ErrorDesc,
			SourceFiles:   append([]string{}, v.SourceFiles...),
		}
	}
	return states
}

func trackFileForEAN(ean, filename string) {
	stateMu.Lock()
	defer stateMu.Unlock()

	if _, exists := eanToFiles[ean]; !exists {
		eanToFiles[ean] = make(map[string]struct{})
	}
	eanToFiles[ean][filename] = struct{}{}
}

func LoadProductState(items []xutil.Item, filename string) []xutil.Item {
	if len(items) == 0 {
		ClearState()
	}

	filteredItems := make([]xutil.Item, 0, len(items))

	for _, it := range items {
		if it.CatalogNumber == "" {
			continue
		}

		key := it.Ean
		if key == "" {
			key = it.CatalogNumber
		}

		trackFileForEAN(key, filename)

		productSt := GetProductState(key)
		if productSt == nil {
			// new product
			errorDesc := ""
			status := uint8(0)

			if it.Ean == "" {
				errorDesc = ErrNoneEAN.Error()
				status = 3
			}

			newState := &ProductState{
				CatalogNumber: it.CatalogNumber,
				Amount:        it.Amount,
				Scanned:       0,
				Difference:    int(-it.Amount),
				Status:        status,
				ErrorDesc:     errorDesc,
				SourceFiles:   []string{filename},
			}

			SetProductState(key, newState)
			filteredItems = append(filteredItems, it)

			if it.Duplicate {
				amountProductsInDuplicate[AmountProductDupKey{
					Ean:      it.Ean,
					Filename: filename,
				}] = it.Amount
			}
			continue
		}

		if it.Duplicate {

			amountProductsInDuplicate[AmountProductDupKey{
				Ean:      it.Ean,
				Filename: filename,
			}] = it.Amount

			if it.IsMainItem {

				sourceFMap := make(map[string]struct{})

				isSameFileDuplicate := false
				for _, f := range productSt.SourceFiles {

					sourceFMap[f] = struct{}{}

					if f == filename && len(productSt.SourceFiles) == 1 {
						isSameFileDuplicate = true
						break
					}
				}

				if isSameFileDuplicate {
					productSt.ErrorDesc = ErrDuplicateSameFile.Error()
				} else if len(productSt.SourceFiles) > 0 {
					_, exists := sourceFMap[filename]
					if !exists {
						productSt.SourceFiles = append(productSt.SourceFiles, filename)
					}

					filesCount := len(productSt.SourceFiles)
					filesList := strings.Join(productSt.SourceFiles, ", ")
					productSt.ErrorDesc = fmt.Sprintf("%s (%d files: %s)", ErrDuplicateDiffFiles.Error(), filesCount, filesList)
				} else {
					productSt.SourceFiles = append(productSt.SourceFiles, filename)
				}

				filteredItems = append(filteredItems, it)
				continue
			}

			if _, exists := alreadyProcessedDuplicates[key]; exists {
				continue
			}

			alreadyProcessedDuplicates[key] = struct{}{}

			// existing product - handle duplicate
			{
				productSt.Amount += it.Amount
				productSt.Difference = int(-productSt.Amount)
				productSt.Status = 3

				sourceFMap0 := make(map[string]struct{})

				isSameFileDuplicate := false
				for _, f := range productSt.SourceFiles {

					sourceFMap0[f] = struct{}{}

					if f == filename && len(productSt.SourceFiles) == 1 {
						isSameFileDuplicate = true
						break
					}
				}

				if isSameFileDuplicate {
					productSt.ErrorDesc = ErrDuplicateSameFile.Error()
				} else if len(productSt.SourceFiles) > 0 {
					_, exists := sourceFMap0[filename]
					if !exists {
						productSt.SourceFiles = append(productSt.SourceFiles, filename)
					}

					filesCount := len(productSt.SourceFiles)
					filesList := strings.Join(productSt.SourceFiles, ", ")
					productSt.ErrorDesc = fmt.Sprintf("%s (%d files: %s)", ErrDuplicateDiffFiles.Error(), filesCount, filesList)
				} else {
					productSt.SourceFiles = append(productSt.SourceFiles, filename)
				}
			}
		} else {
			filteredItems = append(filteredItems, it)
		}
	}

	return filteredItems
}

func ResetScanHistory(filePath string) ([]ProductState, error) {
	res := []ProductState{}

	xmlData, err := xutil.DecodeXMLToStruct(filePath)
	if err != nil {
		return nil, err
	}

	for _, item := range xmlData.Document.Items {
		productSt := GetProductState(item.Ean)
		if productSt != nil {
			productSt.Scanned = 0
			productSt.Difference = int(-productSt.Amount)

			if productSt.ErrorDesc != "" {
				productSt.Status = 3
			} else {
				productSt.Status = 0
			}
			res = append(res, *productSt)
		}
	}

	return res, nil
}

func RemoveFileState(filename string) {
	stateMu.Lock()
	defer stateMu.Unlock()

	for ean, files := range eanToFiles {
		if _, exists := files[filename]; exists {
			delete(files, filename)

			if product, exists := productState[ean]; exists {
				newSourceFiles := make([]string, 0, len(product.SourceFiles))
				for _, f := range product.SourceFiles {
					if f != filename {
						newSourceFiles = append(newSourceFiles, f)
					}
				}
				product.SourceFiles = newSourceFiles

				if len(newSourceFiles) == 0 {
					delete(productState, ean)
					delete(alreadyProcessedDuplicates, ean)
				} else {
					if strings.Contains(product.ErrorDesc, filename) {
						if len(newSourceFiles) > 1 {
							filesCount := len(newSourceFiles)
							filesList := strings.Join(newSourceFiles, ", ")
							product.ErrorDesc = fmt.Sprintf("%s (%d files: %s)",
								ErrDuplicateDiffFiles.Error(), filesCount, filesList)
						} else {
							amount, exists := amountProductsInDuplicate[AmountProductDupKey{
								Ean:      ean,
								Filename: filename,
							}]
							if !exists {
								continue
							}
							product.Amount -= amount
							product.ErrorDesc = ""

							product.Difference = int(product.Scanned - uint(product.Amount))

							if product.Difference == 0 {
								product.Status = 1
							} else if product.Difference < 0 && product.Scanned != 0 {
								product.Status = 2
							} else if product.Difference > 0 {
								product.Status = 3
							} else {
								product.Status = 0
							}
						}
					}
				}
			}

			if len(files) == 0 {
				delete(eanToFiles, ean)
			}
		}
	}

	delete(xutil.XmlItemsFromFilename, filename)
}

func ClearState() {
	stateMu.Lock()
	defer stateMu.Unlock()
	productState = make(map[string]*ProductState)
	eanToFiles = make(map[string]map[string]struct{})
	alreadyProcessedDuplicates = make(map[string]struct{})
}
