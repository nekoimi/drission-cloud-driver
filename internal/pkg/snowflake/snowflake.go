package snowflake

import (
	"fmt"

	sf "github.com/bwmarrin/snowflake"
)

var node *sf.Node

// Init 初始化雪花算法节点
func Init(nodeID int64) error {
	var err error
	node, err = sf.NewNode(nodeID)
	if err != nil {
		return fmt.Errorf("failed to create snowflake node: %w", err)
	}
	return nil
}

// GenerateID 生成雪花 ID
func GenerateID() sf.ID {
	return node.Generate()
}

// GenerateStringID 生成雪花 ID 字符串
func GenerateStringID() string {
	return node.Generate().String()
}
