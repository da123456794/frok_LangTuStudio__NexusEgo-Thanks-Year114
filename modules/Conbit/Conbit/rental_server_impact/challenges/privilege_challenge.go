package challenges

import (
	"context"
	"fmt"
	"time"

	"github.com/LangTuStudio/Conbit/Conbit"
	"github.com/LangTuStudio/Conbit/i18n"
	"github.com/LangTuStudio/Conbit/internal/termlog"
	"github.com/LangTuStudio/Conbit/minecraft/protocol"
	"github.com/LangTuStudio/Conbit/minecraft/protocol/packet"
)

type OperatorChallenge struct {
	Conbit.MicroOmega
	hasOpPrivilege  bool
	cheatOn         bool
	lostPrivilegeCB func()
}

func challengeInfoLine(msg string) {
	termlog.Infof("%s", msg)
}

func challengeInfof(format string, args ...any) {
	termlog.Infof(format, args...)
}

func challengeDoneLine(msg string) {
	termlog.Successf("%s", msg)
}

func NewOperatorChallenge(omega Conbit.MicroOmega, lostPrivilegeCallBack func()) *OperatorChallenge {
	if lostPrivilegeCallBack == nil {
		lostPrivilegeCallBack = func() {
			omega.CloseWithError(fmt.Errorf("%s", i18n.T(i18n.S_bot_op_privilege_removed)))
		}
	}
	helper := &OperatorChallenge{
		MicroOmega:      omega,
		lostPrivilegeCB: lostPrivilegeCallBack,
	}
	omega.GetGameListener().AddNewNoBlockAndDetachablePacketCallBack(
		map[uint32]bool{
			packet.IDAddPlayer:       true,
			packet.IDUpdateAbilities: true,
		},
		func(pk packet.Packet) error {
			switch pk.ID() {
			case packet.IDAddPlayer:
				pkt := pk.(*packet.AddPlayer)
				helper.onPermissionChange(pkt.AbilityData)
			case packet.IDUpdateAbilities:
				pkt := pk.(*packet.UpdateAbilities)
				helper.onPermissionChange(pkt.AbilityData)
			}
			return nil
		},
	)
	omega.GetGameListener().SetTypedPacketCallBack(packet.IDSetCommandsEnabled, helper.onSetCommandEnabledPacket, false)
	return helper
}

func (o *OperatorChallenge) onPermissionChange(abilityData protocol.AbilityData) {
	if o.GetMicroUQHolder().GetBotBasicInfo().GetBotUniqueID() == abilityData.EntityUniqueID {
		if abilityData.CommandPermissions >= packet.CommandPermissionLevelHost {
			o.hasOpPrivilege = true
		} else {
			if o.hasOpPrivilege {
				o.lostPrivilegeCB()
			}
			o.hasOpPrivilege = false
		}
	}
}

func (o *OperatorChallenge) onSetCommandEnabledPacket(pk packet.Packet) {
	p := pk.(*packet.SetCommandsEnabled)
	o.cheatOn = p.Enabled
	if !o.cheatOn && o.hasOpPrivilege {
		o.GetGameControl().SendWOCmd("changesetting allow-cheats true")
		o.cheatOn = true
	}
}

func (o *OperatorChallenge) WaitForPrivilege(ctx context.Context) (err error) {
	for !o.hasOpPrivilege {
		time.Sleep(1 * time.Second)
		o.GetGameControl().SendWOCmd("changesetting allow-cheats true")
		if ret, err := o.GetGameControl().SendWebSocketCmdNeedResponse("tp @s ~~~").SetTimeout(3 * time.Second).BlockGetResult(); err == nil && ret != nil && ret.SuccessCount > 0 {
			o.hasOpPrivilege = true
			o.cheatOn = true
		}
		if ctx.Err() != nil {
			return fmt.Errorf("%s", i18n.T(i18n.S_bot_operator_privilege_timeout))
		}
		if !o.hasOpPrivilege {
			// o.GetGameControl().BotSay(i18n.T(i18n.S_please_grant_bot_operator_privilege))
			challengeInfof(i18n.T(i18n.S_please_grant_bot_operator_privilege), termlog.GradientText(o.GetMicroUQHolder().GetBotBasicInfo().GetBotName()))
		}
	}
	challengeDoneLine(i18n.T(i18n.S_bot_operator_privilege_granted))
	return nil
}
