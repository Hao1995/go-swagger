package goas

import (
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
)

type Goas struct {
	GoPaths       []string
	CurrentGoPath string
	PackageName   string

	OASSpec *OASSpecObject

	CurrentPackage   string
	PackagePathCache map[string]string
	TypeDefinitions  map[string]map[string]*ast.TypeSpec
	FuncDefinitions  map[string]bool //Harry
	PackagesCache    map[string]map[string]*ast.Package
	PackageImports   map[string]map[string][]string

	EnableDebug bool
}

func New() *Goas {
	pwd, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		log.Fatal("$GOPATH environment variable is empty.")
	}

	// pwd = "c:\\gotool\\src\\gitlab.paradise-soft.com.tw\\backend\\goas\\example"
	// gopath = strings.ToLower(gopath) //Harry

	// pwd = "c:\\gotool\\src\\gitlab.paradise-soft.com.tw\\routing\\apis\\mock" //Harry
	// gopath = strings.ToLower(gopath)                                          //Harry

	// pwd = "c:\\gotool\\src\\gitlab.paradise-soft.com.tw\\backend\\dwh" //Harry
	// gopath = strings.ToLower(gopath)                                   //Harry

	gopaths := strings.Split(gopath, ":")
	if runtime.GOOS == "windows" {
		gopaths = strings.Split(gopath, ";")
	}

	currentGopath := ""
	packageName := ""

	for _, p := range gopaths {
		if strings.HasPrefix(pwd, p) {
			currentGopath = p
			packageName = strings.TrimLeft(strings.TrimPrefix(pwd, filepath.Join(p, "src")), string(filepath.Separator))
			break
		}
	}

	if packageName == "" {
		log.Fatalf("Can not find your current package name under GOPATH: %s", gopath)
	}

	g := &Goas{
		GoPaths:          gopaths,
		CurrentGoPath:    currentGopath,
		PackageName:      packageName,
		OASSpec:          &OASSpecObject{},
		PackagePathCache: map[string]string{},
		TypeDefinitions:  map[string]map[string]*ast.TypeSpec{},
		FuncDefinitions:  map[string]bool{},
		PackagesCache:    map[string]map[string]*ast.Package{},
		PackageImports:   map[string]map[string][]string{},
	}

	return g
}

// CreateOASFile outputs OAS file.
func (g *Goas) CreateOASFile(path string) error {
	fd, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("Can not create the master index.json file: %v", err)
	}
	defer fd.Close()

	g.parseInfo()

	g.parseAPIs()

	output, err := json.MarshalIndent(g.OASSpec, "", "  ")
	if err != nil {
		return err
	}
	_, err = fd.WriteString(string(output))

	return err
}

func (g *Goas) debug(v ...interface{}) {
	if g.EnableDebug {
		log.Println(v...)
	}
}

func (g *Goas) debugf(format string, args ...interface{}) {
	if g.EnableDebug {
		log.Printf(format+"\n", args...)
	}
}

// parseInfo parses data for InfoObject from main file.
func (g *Goas) parseInfo() {
	// Find main file
	mainFilename := ""
	goFilenames, err := filepath.Glob("*.go")
	if err != nil {
		log.Fatal(err)
	}
	for _, goFilename := range goFilenames {
		if isMainFile(goFilename) {
			mainFilename = goFilename
			break
		}
	}
	if mainFilename == "" {
		log.Fatal("can not find the main file in the current folder")
	}
	g.debugf("main file: %s", mainFilename)

	// Parse main file comments
	fileSet := token.NewFileSet()
	fileTree, err := goparser.ParseFile(fileSet, mainFilename, nil, goparser.ParseComments)
	if err != nil {
		log.Fatalf("Can not parse general API information: %v\n", err)
	}

	g.OASSpec.OnpenAPI = OpenAPIVersion
	g.OASSpec.Info = &InfoObject{}
	g.OASSpec.Paths = map[string]*PathItemObject{}

	if fileTree.Comments != nil {
		for _, comment := range fileTree.Comments {
			for _, commentLine := range strings.Split(comment.Text(), "\n") {
				attribute := strings.ToLower(strings.Split(commentLine, " ")[0])
				switch attribute {
				case "@serversurl":
					g.OASSpec.Servers = []*ServerObject{{
						URL: strings.TrimSpace(commentLine[len(attribute):]),
					}}
				case "@version":
					g.OASSpec.Info.Version = strings.TrimSpace(commentLine[len(attribute):])
				case "@title":
					g.OASSpec.Info.Title = strings.TrimSpace(commentLine[len(attribute):])
				case "@description":
					g.OASSpec.Info.Description = strings.TrimSpace(commentLine[len(attribute):])
				case "@termsofserviceurl":
					g.OASSpec.Info.TermsOfService = strings.TrimSpace(commentLine[len(attribute):])
				case "@contactname":
					if g.OASSpec.Info.Contact == nil {
						g.OASSpec.Info.Contact = &ContactObject{}
					}
					g.OASSpec.Info.Contact.Name = strings.TrimSpace(commentLine[len(attribute):])
				case "@contactemail":
					if g.OASSpec.Info.Contact == nil {
						g.OASSpec.Info.Contact = &ContactObject{}
					}
					g.OASSpec.Info.Contact.Email = strings.TrimSpace(commentLine[len(attribute):])
				case "@contacturl":
					if g.OASSpec.Info.Contact == nil {
						g.OASSpec.Info.Contact = &ContactObject{}
					}
					g.OASSpec.Info.Contact.URL = strings.TrimSpace(commentLine[len(attribute):])
				case "@licensename":
					if g.OASSpec.Info.License == nil {
						g.OASSpec.Info.License = &LicenseObject{}
					}
					g.OASSpec.Info.License.Name = strings.TrimSpace(commentLine[len(attribute):])
				case "@licenseurl":
					if g.OASSpec.Info.License == nil {
						g.OASSpec.Info.License = &LicenseObject{}
					}
					g.OASSpec.Info.License.URL = strings.TrimSpace(commentLine[len(attribute):])
				}
			}
		}
	}
}

// parseAPI parses apis data.
func (g *Goas) parseAPIs() {
	splitedPackageNames := strings.Split(g.PackageName, "/")
	layerPackageNames := []string{}
	for i := range splitedPackageNames {
		layerPackageNames = append(layerPackageNames, filepath.Join(splitedPackageNames[:i+1]...))
	}
	sort.Slice(layerPackageNames, func(i, j int) bool {
		return len(layerPackageNames[i]) > len(layerPackageNames[j])
	})

	// Maybe refine the layerPackageNames later...

	packageNames := g.scanPackages(layerPackageNames)

	for _, packageName := range packageNames {
		g.parseTypeDefinitions(packageName)
	}
	for _, packageName := range packageNames {
		g.parsePaths(packageName)
	}
}

// scanPackages scans packages and returns them.
func (g *Goas) scanPackages(packages []string) []string {
	res := []string{}
	existsPackages := map[string]bool{}

	for _, packageName := range packages {
		_, ok := existsPackages[packageName]
		if !ok {
			g.debug("found", packageName)
			res = append(res, packageName)
			existsPackages[packageName] = true

			// Get it's real path
			pkgRealPath := g.getRealPackagePath(packageName)

			// Then walk
			var walker filepath.WalkFunc = func(path string, info os.FileInfo, err error) error {
				if info.IsDir() {
					idx := strings.Index(path, packageName)
					if idx != -1 {
						pack := path[idx:]
						_, ok := existsPackages[pack]
						if !ok && pack != "" {
							g.debug("found", pack)
							res = append(res, pack)
							existsPackages[pack] = true
						}
					}
				}
				return nil
			}
			filepath.Walk(pkgRealPath, walker)
		}
	}
	return res
}

// getRealPackagePath try to get real path of a package.
func (g *Goas) getRealPackagePath(packagePath string) string {
	packagePath = strings.Trim(packagePath, "\"")

	// packagePathName := strings.Replace(packagePath, "\\", "-", -1)
	cachedPackagePath, ok := g.PackagePathCache[packagePath]
	if ok {
		return cachedPackagePath
	}

	pkgRealpath := ""

	// Check under GOPATHs and vendor
	for _, goPath := range g.GoPaths {
		splitedPackageNames := strings.Split(g.PackageName, "/")
		layerPackageNames := []string{}
		for i := range splitedPackageNames {
			layerPackageNames = append(layerPackageNames, filepath.Join(splitedPackageNames[:i+1]...))
		}
		sort.Slice(layerPackageNames, func(i, j int) bool {
			return len(layerPackageNames[i]) > len(layerPackageNames[j])
		})

		for _, pn := range layerPackageNames {
			filePath := filepath.Join(goPath, "src", pn, "vendor", packagePath)
			evalutedPath, err := filepath.EvalSymlinks(filePath)
			if err == nil {
				_, err = os.Stat(evalutedPath)
				if err == nil {
					pkgRealpath = evalutedPath
					break
				}
			}
		}
		if pkgRealpath != "" {
			break
		}

		evalutedPath, err := filepath.EvalSymlinks(filepath.Join(goPath, "src", g.PackageName, "vendor", packagePath))
		if err == nil {
			_, err = os.Stat(evalutedPath)
			if err == nil {
				pkgRealpath = evalutedPath
				break
			}
		} else {
			evalutedPath, err = filepath.EvalSymlinks(filepath.Join(goPath, "src", packagePath))
			if err == nil {
				_, err := os.Stat(evalutedPath)
				if err == nil {
					pkgRealpath = evalutedPath
					break
				}
			}
		}
	}

	// Check under $GOROOT/src, $GOROOT/src/vendor and $GOROOT/src/pkg (for golang < v1.4)
	if pkgRealpath == "" {
		goRoot := filepath.Clean(runtime.GOROOT())
		if goRoot == "" {
			log.Fatalf("Please, set $GOROOT environment variable\n")
		}
		evalutedPath, err := filepath.EvalSymlinks(filepath.Join(goRoot, "src", "vendor", packagePath))
		if err == nil {
			_, err := os.Stat(evalutedPath)
			if err == nil {
				pkgRealpath = evalutedPath
			}
		} else {
			evalutedPath, err = filepath.EvalSymlinks(filepath.Join(goRoot, "src", packagePath))
			if err == nil {
				_, err = os.Stat(evalutedPath)
				if err == nil {
					pkgRealpath = evalutedPath
				}
			}
		}

		if pkgRealpath == "" {
			evalutedPath, err = filepath.EvalSymlinks(filepath.Join(goRoot, "src", "pkg", packagePath))
			if err == nil {
				_, err = os.Stat(evalutedPath)
				if err == nil {
					pkgRealpath = evalutedPath
				}
			}
		}
	}

	if pkgRealpath == "" {
		g.debugf("Can not find package %s", packagePath)
	}

	g.PackagePathCache[packagePath] = pkgRealpath

	return pkgRealpath
}

func (g *Goas) parseTypeDefinitions(packageName string) {

	g.CurrentPackage = packageName
	pkgRealPath := g.getRealPackagePath(packageName)

	if pkgRealPath == "" {
		return
	}
	//	log.Printf("Parse type definition of %#v\n", packageName)

	//Harry
	_, ok := g.TypeDefinitions[pkgRealPath]
	if !ok {
		g.TypeDefinitions[pkgRealPath] = map[string]*ast.TypeSpec{}
	}
	if strings.HasSuffix(pkgRealPath, "core") { //Harry: Filter "core" package
		return
	}
	//Harry

	astPackages := g.getPackageAst(pkgRealPath)
	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astDeclaration := range astFile.Decls {
				if generalDeclaration, ok := astDeclaration.(*ast.GenDecl); ok && generalDeclaration.Tok == token.TYPE {
					for _, astSpec := range generalDeclaration.Specs {
						if typeSpec, ok := astSpec.(*ast.TypeSpec); ok {
							g.TypeDefinitions[pkgRealPath][typeSpec.Name.String()] = typeSpec
						}
					}
				}
			}
		}
	}

	for importedPackage := range g.parseImportStatements(packageName) {
		g.parseTypeDefinitions(importedPackage)
	}
}

func (g *Goas) getPackageAst(packagePath string) map[string]*ast.Package {
	//log.Printf("Parse %s package\n", packagePath)
	if cache, ok := g.PackagesCache[packagePath]; ok {
		return cache
	} else {
		fileSet := token.NewFileSet()

		astPackages, err := goparser.ParseDir(fileSet, packagePath, parserFileFilter, goparser.ParseComments)
		if err != nil {
			log.Fatalf("Parse of \"%s\" pkg cause error: %s", packagePath, err)
		}
		g.PackagesCache[packagePath] = astPackages
		return astPackages
	}
}

// parserFileFilter filters dir, hidden file and test file.
func parserFileFilter(info os.FileInfo) bool {
	name := info.Name()
	return !info.IsDir() && !strings.HasPrefix(name, ".") && strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go")
}

// parseImportStatements parses the imported packages of packageName.
func (g *Goas) parseImportStatements(packageName string) map[string]bool {

	g.CurrentPackage = packageName
	pkgRealPath := g.getRealPackagePath(packageName)

	imports := map[string]bool{}
	astPackages := g.getPackageAst(pkgRealPath)

	g.PackageImports[pkgRealPath] = map[string][]string{}
	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astImport := range astFile.Imports {
				importedPackageName := strings.Trim(astImport.Path.Value, "\"")
				realPath := g.getRealPackagePath(importedPackageName)
				if _, ok := g.TypeDefinitions[realPath]; !ok {
					imports[importedPackageName] = true
				}

				// Deal with alias of imported package
				var importedPackageAlias string
				if astImport.Name != nil && astImport.Name.Name != "." && astImport.Name.Name != "_" {
					importedPackageAlias = astImport.Name.Name
				} else {
					importPath := strings.Split(importedPackageName, "/")
					importedPackageAlias = importPath[len(importPath)-1]
				}

				isExists := false
				for _, v := range g.PackageImports[pkgRealPath][importedPackageAlias] {
					if v == importedPackageName {
						isExists = true
					}
				}
				if !isExists {
					g.PackageImports[pkgRealPath][importedPackageAlias] = append(g.PackageImports[pkgRealPath][importedPackageAlias], importedPackageName)
				}
			}
		}
	}

	return imports
}

// parseImportStatements : by Harry. just be used by parsePath.
func (g *Goas) parsePathImportStatements(packageName string) map[string]bool {

	g.CurrentPackage = packageName
	pkgRealPath := g.getRealPackagePath(packageName)

	imports := map[string]bool{}
	astPackages := g.getPackageAst(pkgRealPath)

	g.PackageImports[pkgRealPath] = map[string][]string{}
	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astImport := range astFile.Imports {
				importedPackageName := strings.Trim(astImport.Path.Value, "\"")
				//Harry
				realPath := g.getRealPackagePath(importedPackageName)
				if _, ok := g.FuncDefinitions[realPath]; !ok {
					imports[importedPackageName] = true
				}
				//Harry

				// Deal with alias of imported package
				var importedPackageAlias string
				if astImport.Name != nil && astImport.Name.Name != "." && astImport.Name.Name != "_" {
					importedPackageAlias = astImport.Name.Name
				} else {
					importPath := strings.Split(importedPackageName, "/")
					importedPackageAlias = importPath[len(importPath)-1]
				}

				isExists := false
				for _, v := range g.PackageImports[pkgRealPath][importedPackageAlias] {
					if v == importedPackageName {
						isExists = true
					}
				}
				if !isExists {
					g.PackageImports[pkgRealPath][importedPackageAlias] = append(g.PackageImports[pkgRealPath][importedPackageAlias], importedPackageName)
				}
			}
		}
	}

	return imports
}

func (g *Goas) parsePaths(packageName string) {

	g.CurrentPackage = packageName
	pkgRealPath := g.getRealPackagePath(packageName)

	if pkgRealPath == "" {
		return
	}
	//Harry === Check this 'pkgRealPath' exists or not.
	_, ok := g.FuncDefinitions[pkgRealPath]
	if !ok {
		g.FuncDefinitions[pkgRealPath] = true
	}
	if strings.HasSuffix(pkgRealPath, "core") { //Harry: Filter "core" package
		return
	}
	//Harry

	astPackages := g.getPackageAst(pkgRealPath)

	for _, astPackage := range astPackages {
		for _, astFile := range astPackage.Files {
			for _, astDescription := range astFile.Decls {
				switch astDeclaration := astDescription.(type) {
				case *ast.FuncDecl:
					operation := &OperationObject{
						Responses: map[string]*ResponseObject{},
					}

					if astDeclaration.Doc != nil && astDeclaration.Doc.List != nil {
						for _, comment := range astDeclaration.Doc.List {
							err := g.parseOperation(operation, packageName, comment.Text)
							if err != nil {
								log.Printf("Can not parse comment for function: %v, package: %v, got error: %v\n", astDeclaration.Name.String(), packageName, err)
							}
						}
					}
					// if operation.Path != "" {
					// 	// parser.AddOperation(operation)
					// }
				}
			}
			// for _, astComment := range astFile.Comments {
			// 	for _, commentLine := range strings.Split(astComment.Text(), "\n") {
			// 		parser.ParseSubApiDescription(commentLine)
			// 	}
			// }
		}
	}
	//Harry === Parse import path
	for importedPackage := range g.parsePathImportStatements(packageName) {
		g.parsePaths(importedPackage)
	}
	//Harry
}

func (g *Goas) parseOperation(operation *OperationObject, packageName, comment string) error {

	commentLine := strings.TrimSpace(strings.TrimLeft(comment, "//"))
	if len(commentLine) == 0 {
		return nil
	}
	attribute := strings.Fields(commentLine)[0]
	switch strings.ToLower(attribute) {
	case "@title":
		operation.Summary = strings.TrimSpace(commentLine[len(attribute):])
	case "@description":
		operation.Description = strings.TrimSpace(commentLine[len(attribute):])
	case "@param":
		err := g.parseParamComment(operation, strings.TrimSpace(commentLine[len(attribute):]))
		if err != nil {
			return err
		}
	case "@paramstruct":
		err := g.parseParamStructComment(operation, strings.TrimSpace(commentLine[len(attribute):]))
		if err != nil {
			return err
		}
	case "@success", "@failure":
		err := g.parseResponseComment(operation, strings.TrimSpace(commentLine[len(attribute):]))
		if err != nil {
			return err
		}
	case "@resource":
		resource := strings.TrimSpace(commentLine[len(attribute):])
		if resource == "" {
			resource = "others"
		}
		if !isInStringList(operation.Tags, resource) {
			operation.Tags = append(operation.Tags, resource)
		}
	case "@router":
		err := g.parseRouteComment(operation, commentLine)
		if err != nil {
			return err
		}
	}

	return nil
}

func (g *Goas) parseParamComment(operation *OperationObject, commentLine string) error {
	paramString := commentLine

	re := regexp.MustCompile(`([-\w]+)[\s]+([\w]+)[\s]+([\w.]+)[\s]+([\w]+)[\s]+"([^"]+)"`)

	matches := re.FindStringSubmatch(paramString)
	if len(matches) != 6 {
		return fmt.Errorf("Can not parse param comment \"%s\", skipped", paramString)
	}

	typeName, err := g.registerType(matches[3])
	if err != nil {
		return err
	}

	parameter := &ParameterObject{}
	parameter.Name = matches[1]
	parameter.In = matches[2]

	if parameter.In != "body" {
		if isBasicTypeOASType(typeName) {
			parameter.Schema = &SchemaObject{
				Type:   basicTypesOASTypes[typeName],
				Format: basicTypesOASFormats[typeName],
			}
		} else {
			_, ok := g.OASSpec.Components.Schemas[convertRefName(typeName)]
			if ok {
				parameter.Schema = &SchemaObject{
					Ref: referenceLink(typeName),
				}
			} else {
				parameter.Schema = &SchemaObject{
					Type: typeName,
				}
			}
		}

		requiredText := strings.ToLower(matches[4])
		parameter.Required = (requiredText == "true" || requiredText == "required")
		if parameter.In == "path" {
			parameter.Required = true
		}

		parameter.Description = matches[5]

		operation.Parameters = append(operation.Parameters, parameter)

		return nil
	}

	operation.RequestBody = &RequestBodyObject{
		Content:  map[string]*MediaTypeObject{},
		Required: true,
	}
	operation.RequestBody.Content[ContentTypeJson] = &MediaTypeObject{}

	_, ok := g.OASSpec.Components.Schemas[convertRefName(typeName)]
	if ok {
		operation.RequestBody.Content[ContentTypeJson].Schema = &SchemaObject{
			Ref: referenceLink(typeName),
		}
	} else {
		operation.RequestBody.Content[ContentTypeJson].Schema = &SchemaObject{
			Type: strings.Trim(matches[2], "{}"),
		}
	}

	if matches[2] == "{array}" {
		operation.RequestBody.Content[ContentTypeJson].Schema = &SchemaObject{
			Type: "array",
			Items: &ReferenceObject{
				Ref: referenceLink(typeName),
				// Ref: typeName,
			},
		}
	} else if operation.RequestBody.Content[ContentTypeJson].Schema.Ref == "" {
		operation.RequestBody.Content[ContentTypeJson].Schema.Type = typeName
	}

	return nil
}

func (g *Goas) parseParamStructComment(operation *OperationObject, commentLine string) error {
	paramString := commentLine

	re := regexp.MustCompile(`([\w.]+)`) //Harry: @ParamStruct reporting.DailyReporting

	matches := re.FindStringSubmatch(paramString)
	if len(matches) != 2 {
		return fmt.Errorf("Can not parse paramStruct comment \"%s\", skipped", paramString)
	}

	params, err := g.registerTypeToParamStruct(matches[1])
	if err != nil {
		return err
	}

	operation.Parameters = append(operation.Parameters, params...)

	return nil
}

// parseRouteComment adds operationObject to PathsObject.
func (g *Goas) parseRouteComment(operation *OperationObject, commentLine string) error {
	sourceString := strings.TrimSpace(commentLine[len("@Router"):])

	re := regexp.MustCompile(`([\w\.\/\-{}]+)[^\[]+\[([^\]]+)`)
	var matches []string

	matches = re.FindStringSubmatch(sourceString)
	if len(matches) != 3 {
		return fmt.Errorf("Can not parse router comment \"%s\", skipped", commentLine)
	}

	_, ok := g.OASSpec.Paths[matches[1]]
	if !ok {
		g.OASSpec.Paths[matches[1]] = &PathItemObject{}
	}

	switch strings.ToUpper(matches[2]) {
	case "GET":
		if g.OASSpec.Paths[matches[1]].Get == nil {
			g.OASSpec.Paths[matches[1]].Get = operation
		}
	case "POST":
		if g.OASSpec.Paths[matches[1]].Post == nil {
			g.OASSpec.Paths[matches[1]].Post = operation
		}
	case "PATCH":
		if g.OASSpec.Paths[matches[1]].Patch == nil {
			g.OASSpec.Paths[matches[1]].Patch = operation
		}
	case "PUT":
		if g.OASSpec.Paths[matches[1]].Put == nil {
			g.OASSpec.Paths[matches[1]].Put = operation
		}
	case "DELETE":
		if g.OASSpec.Paths[matches[1]].Delete == nil {
			g.OASSpec.Paths[matches[1]].Delete = operation
		}
	case "OPTIONS":
		if g.OASSpec.Paths[matches[1]].Options == nil {
			g.OASSpec.Paths[matches[1]].Options = operation
		}
	case "HEAD":
		if g.OASSpec.Paths[matches[1]].Head == nil {
			g.OASSpec.Paths[matches[1]].Head = operation
		}
	case "TRACE":
		if g.OASSpec.Paths[matches[1]].Trace == nil {
			g.OASSpec.Paths[matches[1]].Trace = operation
		}
	}

	return nil
}

// parseResponseComment
func (g *Goas) parseResponseComment(operation *OperationObject, commentLine string) error {
	re := regexp.MustCompile(`([\d]+)[\s]+([\w\{\}]+)[\s]+([\w\-\.\/]+)[^"]*(.*)?`)
	var matches []string

	matches = re.FindStringSubmatch(commentLine)
	if len(matches) != 5 {
		return fmt.Errorf("Can not parse response comment \"%s\", skipped", commentLine)
	}

	var response *ResponseObject
	var code int
	code, err := strconv.Atoi(matches[1])
	if err != nil {
		return errors.New("Success http code must be int")
	} else {
		operation.Responses[fmt.Sprint(code)] = &ResponseObject{
			Content: map[string]*MediaTypeObject{},
		}
		response = operation.Responses[fmt.Sprint(code)]
		response.Content[ContentTypeJson] = &MediaTypeObject{}
	}
	response.Description = strings.Trim(matches[4], "\"")

	typeName, err := g.registerType(matches[3])
	if err != nil {
		return err
	}

	_, ok := g.OASSpec.Components.Schemas[convertRefName(typeName)]
	if ok {
		response.Content[ContentTypeJson].Schema = &SchemaObject{
			Ref: referenceLink(typeName),
		}
	} else {
		response.Content[ContentTypeJson].Schema = &SchemaObject{
			Type: strings.Trim(matches[2], "{}"),
		}
	}

	if matches[2] == "{array}" {
		response.Content[ContentTypeJson].Schema = &SchemaObject{
			Type: "array",
			Items: &ReferenceObject{
				Ref: referenceLink(typeName),
			},
		}
	} else if response.Content[ContentTypeJson].Schema.Ref == "" {
		response.Content[ContentTypeJson].Schema.Type = typeName
	}

	// output, err := json.MarshalIndent(response, "", "  ")
	// fmt.Println(string(output))

	return nil
}

func (g *Goas) registerType(typeName string) (string, error) {
	registerType := ""

	translation, ok := typeDefTranslations[typeName]
	if ok {
		registerType = translation
	} else if isBasicType(typeName) {
		registerType = typeName
	} else {
		model := &Model{}
		knownModelNames := map[string]bool{}

		innerModels, err := g.parseModel(model, typeName, g.CurrentPackage, knownModelNames)
		if err != nil {
			return registerType, err
		}
		if translation, ok := typeDefTranslations[typeName]; ok {
			registerType = translation
		} else {
			registerType = model.Id

			if g.OASSpec.Components == nil {
				g.OASSpec.Components = &ComponentsOjbect{
					Schemas:    map[string]*SchemaObject{},
					Responses:  map[string]*ResponseObject{},
					Parameters: map[string]*ParameterObject{},
				}
			}

			//Harry: 這邊的registerType是"github.com. ... .example.model.Data"，所以好像不用convert
			// componentsSchemasName := strings.Replace(registerType, "\\", "-", -1)
			componentsSchemasName := convertRefName(registerType)
			_, ok := g.OASSpec.Components.Schemas[componentsSchemasName]
			if !ok {
				g.OASSpec.Components.Schemas[componentsSchemasName] = &SchemaObject{
					Type:       "object",
					Required:   model.Required,
					Properties: map[string]interface{}{},
				}
			}

			for k, v := range model.Properties { //Harry: 塞property給schemas
				if v.Ref != "" {
					v.Type = ""
					v.Items = nil
					v.Format = ""
				}
				g.OASSpec.Components.Schemas[componentsSchemasName].Properties[k] = v
			}

			//Harry === Be used to parse the type directly equal other type
			if len(model.ExtraModel) > 0 {
				innerModels = append(innerModels, model.ExtraModel...)
			}
			//Harry ===
			for _, m := range innerModels {
				registerType := m.Id
				componentsSchemasName := convertRefName(registerType) //Harry: 似乎是不用轉換，因為m.Id本來就是轉換過的
				if _, ok := g.OASSpec.Components.Schemas[componentsSchemasName]; !ok {
					g.OASSpec.Components.Schemas[componentsSchemasName] = &SchemaObject{
						Type:       "object",
						Required:   m.Required,
						Properties: map[string]interface{}{},
					}
				}
				for k, v := range m.Properties {
					// if v.AllOf != ""{
					// 	v.Type = ""
					// 	v.Items = nil
					// 	v.Format = ""
					// 	v.AllOf = ""

					// allOfobj := &AllOfObj{}
					// allOfobj.
					// g.OASSpec.Components.Schemas[componentsSchemasName].AllOf
					// }else{
					if v.Ref != "" {
						v.Type = ""
						v.Items = nil
						v.Format = ""
					}
					g.OASSpec.Components.Schemas[componentsSchemasName].Properties[k] = v
					// }
				}
			}
		}
	}

	return registerType, nil
}

func (g *Goas) registerTypeToParamStruct(typeName string) ([]*ParameterObject, error) {

	registerType, err := g.registerType(typeName)
	if err != nil {
		return nil, err
	}

	//Harry === Parse params from g.OASSpec.Components.Schemas
	params := []*ParameterObject{}
	componentsSchemasName := convertRefName(registerType)
	if schemaObj, ok := g.OASSpec.Components.Schemas[componentsSchemasName]; ok {
		for paramName, property := range schemaObj.Properties {
			param := &ParameterObject{}
			propertyModel := property.(*ModelProperty)

			//Harry: should declare this func at outside
			findRequired := func(paramName string, schemaObj *SchemaObject) bool {
				flag := false
				for _, requiredName := range schemaObj.Required {
					if requiredName == paramName {
						flag = true
						break
					}
				}
				return flag
			}

			param.Name = paramName
			param.In = "query"
			param.Description = propertyModel.Description
			param.Required = findRequired(paramName, schemaObj)
			param.Schema = &SchemaObject{
				Type:   propertyModel.Type,
				Format: propertyModel.Format,
			}

			params = append(params, param)
		}
	}

	return params, nil
}

type Model struct {
	Id         string                    `json:"id,omitempty"`
	Required   []string                  `json:"required,omitempty"`
	Properties map[string]*ModelProperty `json:"properties,omitempty"`
	Ref        string                    `json:"$ref,omitempty"`

	ExtraModel []*Model `json:"extramodel,omitempty"` //Harry
}

type ModelProperty struct {
	Description string              `json:"description,omitempty"`
	Required    bool                `json:"required,omitempty"`
	Type        string              `json:"type,omitempty"`
	Format      string              `json:"format,omitempty"`
	Items       *ModelPropertyItems `json:"items,omitempty"`
	Ref         string              `json:"$ref,omitempty"`
}

type ModelPropertyItems struct {
	Ref  string `json:"$ref,omitempty"`
	Type string `json:"type,omitempty"`
}

func (g *Goas) parseModel(m *Model, modelName string, currentPackage string, knownModelNames map[string]bool) ([]*Model, error) {
	knownModelNames[modelName] = true

	astTypeSpec, modelPackage := g.findModelDefinition(modelName, currentPackage)

	modelNameParts := strings.Split(modelName, ".")
	m.Id = strings.Join(append(strings.Split(modelPackage, "/"), modelNameParts[len(modelNameParts)-1]), ".")

	_, ok := modelNamesPackageNames[modelName]
	if !ok {
		modelNamesPackageNames[modelName] = m.Id
	}

	var innerModelList []*Model
	astTypeDef, ok := astTypeSpec.Type.(*ast.Ident)
	//Harry: type直接等於basic type
	//EX: type BasicModel = string
	if ok {
		typeDefTranslations[m.Id] = astTypeDef.Name
	} else if astStructType, ok := astTypeSpec.Type.(*ast.StructType); ok { //Harry: 一般Struct型式
		g.parseFieldList(m, astStructType.Fields.List, modelPackage) //Harry: 把此struct下面的fields都parse過
		usedTypes := map[string]bool{}

		for _, property := range m.Properties { //Comment: each normal properties will be mapping
			typeName := property.Type
			if typeName == "array" {
				if property.Items.Type != "" {
					typeName = property.Items.Type
				} else {
					typeName = property.Items.Ref
				}
			}
			if translation, ok := typeDefTranslations[typeName]; ok {
				typeName = translation
			}
			if isBasicType(typeName) {
				if isBasicTypeOASType(typeName) {
					property.Format = basicTypesOASFormats[typeName]
					if property.Type != "array" {
						property.Type = basicTypesOASTypes[typeName]
					} else {
						if isBasicType(property.Items.Type) {
							if isBasicTypeOASType(property.Items.Type) {
								property.Items.Type = basicTypesOASTypes[property.Items.Type]
							}
						}
					}
				}
				continue
			}
			// if g.isImplementMarshalInterface(typeName) {
			// 	continue
			// }
			if _, exists := knownModelNames[typeName]; exists {
				// fmt.Println("@", typeName)
				if _, ok := modelNamesPackageNames[typeName]; ok {
					if translation, ok := typeDefTranslations[modelNamesPackageNames[typeName]]; ok {
						if isBasicType(translation) {
							if isBasicTypeOASType(translation) {
								// fmt.Println(modelNamesPackageNames[typeName], translation)
								property.Type = basicTypesOASTypes[translation]
							}
							continue
						}
					}
					if property.Type != "array" {
						property.Ref = referenceLink(modelNamesPackageNames[typeName])
					} else {
						property.Items.Ref = referenceLink(modelNamesPackageNames[typeName])
					}
				}
				continue
			}

			if property.Ref != "" {
				continue
			}

			usedTypes[typeName] = true
		}

		//log.Printf("Before parse inner model list: %#v\n (%s)", usedTypes, modelName)
		innerModelList = []*Model{}

		for typeName := range usedTypes {
			typeModel := &Model{}
			if typeInnerModels, err := g.parseModel(typeModel, typeName, modelPackage, knownModelNames); err != nil {
				//log.Printf("Parse Inner Model error %#v \n", err)
				return nil, err
			} else {
				for _, property := range m.Properties {
					if property.Type == "array" {
						if property.Items.Ref == typeName {
							property.Items.Ref = referenceLink(typeModel.Id) //Harry: Here should be fixed "/" to "-"
						}
					} else {
						if property.Type == typeName {
							if translation, ok := typeDefTranslations[modelNamesPackageNames[typeName]]; ok {
								if isBasicType(translation) {
									if isBasicTypeOASType(translation) {
										property.Type = basicTypesOASTypes[translation]
									}
									continue
								}
							}
							property.Ref = referenceLink(typeModel.Id)
						} else {
							// fmt.Println(property.Type, "<>", typeName)
						}
					}
				}
				//log.Printf("Inner model %v parsed, parsing %s \n", typeName, modelName)
				if typeModel != nil {
					innerModelList = append(innerModelList, typeModel)
				}
				if typeInnerModels != nil && len(typeInnerModels) > 0 {
					innerModelList = append(innerModelList, typeInnerModels...)
				}
				//log.Printf("innerModelList: %#v\n, typeInnerModels: %#v, usedTypes: %#v \n", innerModelList, typeInnerModels, usedTypes)
			}
		}
		// log.Printf("After parse inner model list: %#v\n (%s)", usedTypes, modelName)
		// log.Fatalf("Inner model list: %#v\n", innerModelList)

	} else if astSelectorExpr, ok := astTypeSpec.Type.(*ast.SelectorExpr); ok {
		//Harry： If this type is directly equal to other type
		//EX: type Hits = globPaging.Hits

		modelNameParts = nil
		if astDataIdent, ok := astSelectorExpr.X.(*ast.Ident); ok {
			//Harry: get package name
			//ex: globPaging
			modelNameParts = append(modelNameParts, astDataIdent.Name)
		}
		modelNameParts = append(modelNameParts, astSelectorExpr.Sel.Name)

		typeModel := &Model{}
		typeInnerModels, err := g.parseModel(typeModel, modelNameParts[0]+"."+modelNameParts[1], modelPackage, knownModelNames)
		if err != nil {
			return nil, err
		}
		//EX: m.Id = "gitlab.paradise-soft.com.tw.platform.common.paging.Hits"的properties應該直接等於"gitlab.paradise-soft.com.tw.glob.utils.paging.Hits"
		// m.Properties = append(m.Properties, typeModel.Properties...)
		if m.Properties == nil {
			m.Properties = make(map[string]*ModelProperty)
		}
		for k, v := range typeModel.Properties {
			m.Properties[k] = v
		}
		m.ExtraModel = append(m.ExtraModel, typeInnerModels...)

	}

	//log.Printf("ParseModel finished %s \n", modelName)
	return innerModelList, nil
}

func (g *Goas) findModelDefinition(modelName string, currentPackage string) (*ast.TypeSpec, string) {
	var model *ast.TypeSpec
	var modelPackage string

	modelNameParts := strings.Split(modelName, ".")

	//if no dot in name - it can be only model from current package
	if len(modelNameParts) == 1 {
		modelPackage = currentPackage
		model = g.getModelDefinition(modelName, currentPackage)
		if model == nil {
			log.Fatalf("Can not find definition of [%s] model. Current package [%s]", modelName, currentPackage)
		}
	} else {
		// First try to assume what name is absolute
		absolutePackageName := strings.Join(modelNameParts[:len(modelNameParts)-1], "/")
		modelNameFromPath := modelNameParts[len(modelNameParts)-1]

		modelPackage = absolutePackageName
		model = g.getModelDefinition(modelNameFromPath, absolutePackageName)
		if model == nil {

			// Can not get model by absolute name.
			if len(modelNameParts) > 2 {
				log.Fatalf("Can not find definition of %s model. Name looks like absolute, but model not found in %s package", modelNameFromPath, absolutePackageName)
			}

			// Lets try to find it in imported packages
			pkgRealPath := g.getRealPackagePath(currentPackage)
			imports, ok := g.PackageImports[pkgRealPath]
			if !ok {
				log.Fatalf("Can not find definition of %s model. Package %s dont import anything", modelNameFromPath, pkgRealPath)
			}
			relativePackage, ok := imports[modelNameParts[0]]
			// if !ok {
			// 	log.Fatalf("Package %s is not imported to %s, Imported: %#v\n", modelNameParts[0], currentPackage, imports)
			// }
			if ok {
				var modelFound bool
				for _, packageName := range relativePackage {
					model = g.getModelDefinition(modelNameFromPath, packageName)
					if model != nil {
						modelPackage = packageName
						modelFound = true

						break
					}
				}
				if !modelFound {
					log.Fatalf("Can not find definition of %s model in package %s", modelNameFromPath, relativePackage)
				}
			} else {
				//Harry: If the model do not import from "currentPackage". Directly find "model" from g.TypeDefinitions

				var modelFound bool

				for modelPkgPath, models := range g.TypeDefinitions {
					if strings.HasSuffix(modelPkgPath, modelNameParts[0]) {
						model = models[modelNameParts[1]]
						modelFound = true
					}
				}
				if !modelFound {
					log.Fatalf("Can not find definition of %s model in GOAS.TypeDefinition", modelNameFromPath)
				}

			}
		}
	}
	return model, modelPackage
}

//Harry: Can not find definition of "string"
func (g *Goas) getModelDefinition(model string, packageName string) *ast.TypeSpec {
	pkgRealPath := g.getRealPackagePath(packageName)
	if pkgRealPath == "" {
		return nil
	}
	packageModels, ok := g.TypeDefinitions[pkgRealPath]
	if !ok {
		return nil
	}
	astTypeSpec, _ := packageModels[model]
	return astTypeSpec
}

func (g *Goas) parseFieldList(m *Model, fieldList []*ast.Field, modelPackage string) {
	if fieldList == nil {
		return
	}

	m.Properties = map[string]*ModelProperty{}
	for _, field := range fieldList {
		g.parseModelProperty(m, field, modelPackage)
	}
}

func (g *Goas) parseModelProperty(m *Model, field *ast.Field, modelPackage string) {
	var name string
	var innerModel *Model

	property := &ModelProperty{}

	typeAsString := getTypeAsString(field.Type)
	//log.Printf("Get type as string %s \n", typeAsString)

	reInternalIndirect := regexp.MustCompile("&\\{(\\w*) <nil> (\\w*)\\}")
	typeAsString = string(reInternalIndirect.ReplaceAll([]byte(typeAsString), []byte("[]$2")))

	// Sometimes reflection reports an object as "&{foo Bar}" rather than just "foo.Bar"
	// The next 2 lines of code normalize them to foo.Bar
	reInternalRepresentation := regexp.MustCompile("&\\{(\\w*) (\\w*)\\}")
	typeAsString = string(reInternalRepresentation.ReplaceAll([]byte(typeAsString), []byte("$1.$2")))

	//Harry: Determine if it's Core
	if strings.Contains(typeAsString, "core.") {
		typeAsString = strings.Replace(typeAsString, "core.", "", -1)
		// if typeAsString == "DateTime" {
		// 	typeAsString = "datetime"
		// } else {
		typeAsString = strings.ToLower(typeAsString)
		// }
	}
	if strings.HasPrefix(typeAsString, "[]") {
		property.Type = "array"
		g.setItemType(property, typeAsString[2:])
		// if is Unsupported item type of list, ignore this property
		if property.Items.Type == "undefined" {
			property = nil
			return
		}
	} else if strings.HasPrefix(typeAsString, "map[]") {
		property.Type = "interface"
	} else if typeAsString == "time.Time" {
		property.Type = "time"
		// property.Type = "datetime"
	} else if typeAsString == "interface" {
		return
	} else {
		property.Type = typeAsString
	}

	if len(field.Names) == 0 { //如果找不到field.Names，可能是繼承其他struct，所以需要去掃這個field的model
		if astSelectorExpr, ok := field.Type.(*ast.SelectorExpr); ok { //
			packageName := modelPackage //Harry: Maybe 'modelPackage' doesn't same with package of this type
			if astTypeIdent, ok := astSelectorExpr.X.(*ast.Ident); ok {
				packageName = astTypeIdent.Name
			}
			name = packageName + "." + strings.TrimPrefix(astSelectorExpr.Sel.Name, "*")
		} else if astTypeIdent, ok := field.Type.(*ast.Ident); ok { //Harry: Normal situation
			name = astTypeIdent.Name
		} else if astStarExpr, ok := field.Type.(*ast.StarExpr); ok { //Harry: Be used by 'pointer'
			if astStarExprX, ok := astStarExpr.X.(*ast.SelectorExpr); ok {
				//Harry: import from other package
				//Ex: *model.Data
				if astDataIdent, ok := astStarExprX.X.(*ast.Ident); ok {
					name = astDataIdent.Name + "." + astStarExprX.Sel.Name
				}
			} else if astTypeIdent, ok := astStarExpr.X.(*ast.Ident); ok {
				//Harry: import from currently package
				//Ex: *Data
				name = astTypeIdent.Name
			}
		} else {
			log.Fatalf("Something goes wrong: %#v", field.Type)
		}
		innerModel = &Model{}
		//log.Printf("Try to parse embeded type %s \n", name)
		//log.Fatalf("DEBUG: field: %#v\n, selector.X: %#v\n selector.Sel: %#v\n", field, astSelectorExpr.X, astSelectorExpr.Sel)
		knownModelNames := map[string]bool{}

		g.parseModel(innerModel, name, modelPackage, knownModelNames)

		for innerFieldName, innerField := range innerModel.Properties {
			m.Properties[innerFieldName] = innerField
		}

		// ===Harry
		if len(innerModel.ExtraModel) > 0 {
			m.ExtraModel = append(m.ExtraModel, innerModel.ExtraModel...)
		}
		// ===Harry

		//log.Fatalf("Here %#v\n", field.Type)
		return
	}
	name = field.Names[0].Name

	//log.Printf("ParseModelProperty: %s, CurrentPackage %s, type: %s \n", name, modelPackage, property.Type)
	//Analyse struct fields annotations
	if field.Tag != nil {
		structTag := reflect.StructTag(strings.Trim(field.Tag.Value, "`"))
		var tagText string
		if thriftTag := structTag.Get("thrift"); thriftTag != "" {
			tagText = thriftTag
		}
		if tag := structTag.Get("json"); tag != "" {
			tagText = tag
		}

		tagValues := strings.Split(tagText, ",")
		var isRequired = false

		for _, v := range tagValues {
			if v != "" && v != "required" && v != "omitempty" {
				name = v
			}
			if v == "required" {
				isRequired = true
			}
			// We will not document at all any fields with a json tag of "-"
			if v == "-" {
				return
			}
		}
		if required := structTag.Get("required"); required != "" || isRequired {
			m.Required = append(m.Required, name)
		}
		if desc := structTag.Get("description"); desc != "" {
			property.Description = desc
		}
	}
	m.Properties[name] = property
}

func (g *Goas) setItemType(p *ModelProperty, itemType string) {
	p.Items = &ModelPropertyItems{}
	if isBasicType(itemType) {
		if isBasicTypeOASType(itemType) {
			p.Items.Type = itemType
		} else {
			p.Items.Type = "undefined"
		}
	} else {
		p.Items.Ref = itemType
	}
}
