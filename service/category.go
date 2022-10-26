package service

import (
	"myecho/dal"
	"myecho/dal/mysql"
)

type CategoryService struct {
}

type Category struct {
	mysql.CategoryModel
	TotalCount uint
}

func (c *CategoryService) All() ([]*Category, error) {

	allMysqlCategories, err := dal.MySqlDB.Category.All()
	if err != nil {
		return nil, err
	}
	categories := make([]*Category, 0, len(allMysqlCategories))
	for _, category := range allMysqlCategories {
		categories = append(categories, &Category{
			CategoryModel: *category,
		})
	}
	FillTotalCountCategories(categories)
	return categories, nil
}

func FillTotalCountCategories(allCategories []*Category) {
	categoryUIDMap := make(map[string]*Category, len(allCategories))
	for _, c := range allCategories {
		categoryUIDMap[c.UID] = c
	}
	helpMap := make(map[string][]*Category, len(allCategories))
	for _, c := range allCategories {
		if _, ok := helpMap[c.UID]; !ok {
			helpMap[c.UID] = make([]*Category, 0)
		}
		if len(c.FatherUID) == 0 {
			continue
		}
		father, exist := categoryUIDMap[c.FatherUID]
		if !exist {
			continue
		}
		if _, ok := helpMap[father.UID]; !ok {
			helpMap[father.UID] = make([]*Category, 0)
		}
		helpMap[father.UID] = append(helpMap[father.UID], c)
	}
	// 计算最终结果
	totalCountMap := make(map[string]uint, len(allCategories))
	stopLoop := func(uid string) bool {
		if _, exist := totalCountMap[uid]; exist {
			return true
		}
		if len(helpMap[uid]) == 0 {
			totalCountMap[uid] = categoryUIDMap[uid].Count
			return true
		}
		return false
	}
	for uid := range helpMap {
		currentUID := uid
		if !stopLoop(currentUID) {
			for i := 0; i < len(helpMap[currentUID]); {
				currentUID = helpMap[currentUID][i].UID
				for !stopLoop(currentUID) {
					// 继续向下第一个, 并清除 i
					currentUID = helpMap[currentUID][0].UID
					i = 0
				}
				// 此时已经是最下一层了, 看下这一层有没有相邻的节点
				if i < len(helpMap[categoryUIDMap[currentUID].FatherUID])-1 {
					i += 1
					currentUID = categoryUIDMap[currentUID].FatherUID
					continue
				}
				// 没有的话就看父节点
				if categoryUIDMap[currentUID].FatherUID != "" {
					// 如果父节点不等于起始 id, 把当前节点移动到父节点, 并计算出父节点的所有值
					currentUID = categoryUIDMap[currentUID].FatherUID
					fatherCount := uint(0)
					for _, c := range helpMap[currentUID] {
						fatherCount += totalCountMap[c.UID]
					}
					fatherCount += categoryUIDMap[currentUID].Count
					totalCountMap[currentUID] = fatherCount
					i = 0
					currentUID = categoryUIDMap[currentUID].FatherUID
					continue
				}
				// 如果不满足的话说明已经遍历完了
				break
			}
		}
	}
	for _, c := range allCategories {
		c.TotalCount = totalCountMap[c.UID]
	}
}
