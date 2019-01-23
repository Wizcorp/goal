package api

type GoalProtoFiles []string

var paths = GoalProtoFiles{}

func RegisterProtoFile(filepath string) {
	paths = append(paths, filepath)
}

func ListProtoFiles() GoalProtoFiles {
	return paths
}
