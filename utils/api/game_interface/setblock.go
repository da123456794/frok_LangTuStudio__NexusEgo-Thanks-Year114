package GameInterface

import (
	"fmt"
	"nexus/utils/api/commands_generator"
	ResourcesControl "nexus/utils/api/resources_control"
	"nexus/defines"
	"nexus/utils/client"
)

// 在 pos 处以 setblock 命令放置名为 name 且方块状态为 states 的方块。
// 此实现是阻塞的，它将等待租赁服回应后再返回值
func (g *GameInterface) SetBlock(pos [3]int32, name string, states string) error {
	request := commands_generator.SetBlockRequest(&types.Module{
		Block: &types.Block{
			Name:        &name,
			BlockStates: states,
		},
		Point: types.Position{
			X: int(pos[0]),
			Y: int(pos[1]),
			Z: int(pos[2]),
		},
	}, &types.MainConfig{}, &client.Client{Cdump_Setting: client.New_Cdump_Setting()})
	// get setblock command
	resp := g.SendWSCommandWithResponse(
		request,
		ResourcesControl.CommandRequestOptions{
			TimeOut: ResourcesControl.CommandRequestDefaultDeadLine,
		},
	)
	if resp.Error != nil && resp.ErrorType == ResourcesControl.ErrCommandRequestTimeOut {
		err := g.SendAICommand(request, true)
		if err != nil {
			return fmt.Errorf("SetBlock: %v", err)
		}
		err = g.AwaitChangesGeneral()
		if err != nil {
			return fmt.Errorf("SetBlock: %v", err)
		}
		return nil
	}
	if resp.Error != nil {
		return fmt.Errorf("SetBlock: %v", resp.Error)
	}
	// send setblock request
	return nil
	// return
}

// 在 pos 处以 setblock 命令放置名为 name 且方块状态为 states 的方块。
// 此实现不会等待租赁服响应，数据包被发送后将立即返回值
func (g *GameInterface) SetBlockAsync(pos [3]int32, name string, states string) error {
	request := commands_generator.SetBlockRequest(&types.Module{
		Block: &types.Block{
			Name:        &name,
			BlockStates: states,
		},
		Point: types.Position{
			X: int(pos[0]),
			Y: int(pos[1]),
			Z: int(pos[2]),
		},
	}, &types.MainConfig{}, &client.Client{Cdump_Setting: client.New_Cdump_Setting()})
	// fmt.Println(request)
	err := g.SendAICommand(request, true)
	if err != nil {
		return fmt.Errorf("SetBlockAsync: %v", err)
	}
	return nil
}
