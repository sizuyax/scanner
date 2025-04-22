package files

import "os"

type File struct {
	Name string
}

func ContentToStruct(dir string) []File {

	files := []File{}

	fileInfos, err := os.ReadDir(dir)
	if err == nil {
		for _, file := range fileInfos {
			files = append(files, File{
				Name: file.Name(),
			})
		}
	}

	return files
}
