package core


func UpdateModuleImportCache(hash string, content string){
	moduleCache[hash] = content
}