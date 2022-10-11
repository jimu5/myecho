package utils

func GetDuplicateSlice[T int | float32 | string](data []T) []T {
	helpMap := make(map[T]struct{})
	result := make([]T, 0, len(data))
	for _, d := range data {
		_, ok := helpMap[d]
		if !ok {
			helpMap[d] = struct{}{}
			result = append(result, d)
		}
	}
	return result
}
