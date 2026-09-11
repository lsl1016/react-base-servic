package credits

import (
	"context"
	"time"

	"react-base-service/components"
	model "react-base-service/models/llm"

	"react-base-service/golib/zlog"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	initialCredits = 1000

	// 换算比例：输入 5000 token = 1 积分，输出 1000 token = 1 积分
	inputTokensPerCredit  = 5000
	outputTokensPerCredit = 1000
)

// currentMonth 返回当前月份字符串，格式 YYYY-MM
func currentMonth() string {
	return time.Now().Format("2006-01")
}

// CalcDeduction 根据 token 数计算需要扣减的积分（向上取整，最少 1 积分）
func CalcDeduction(inputTokens, outputTokens int) int {
	d := inputTokens/inputTokensPerCredit + outputTokens/outputTokensPerCredit
	if d <= 0 {
		d = 1
	}
	return d
}

// GetOrInit 惰性初始化 / 月初重置基础积分，返回当前有效的基础积分值
// 不存在 → 初始化 credits=1000；存在但 reset_month 非当月 → 重置为 1000
func GetOrInit(ctx *gin.Context, userName, modelHash string) (int, error) {
	month := currentMonth()

	record, err := model.GetBaseCredits(ctx, userName, modelHash)
	if err != nil {
		return 0, err
	}

	if record == nil {
		// 首次使用，初始化
		newRecord := &model.UserBaseCredits{
			UserName:   userName,
			ModelHash:  modelHash,
			Credits:    initialCredits,
			ResetMonth: month,
		}
		if err := model.CreateBaseCredits(ctx, newRecord); err != nil {
			return 0, err
		}
		zlog.Infof(ctx, "[credits.GetOrInit] 初始化积分: userName=%s, modelHash=%s, credits=%d", userName, modelHash, initialCredits)
		return initialCredits, nil
	}

	if record.ResetMonth != month {
		// 新月份，惰性重置
		if err := model.UpdateBaseCredits(ctx, userName, modelHash, map[string]interface{}{
			"credits":     initialCredits,
			"reset_month": month,
		}); err != nil {
			return 0, err
		}
		zlog.Infof(ctx, "[credits.GetOrInit] 月初重置积分: userName=%s, modelHash=%s, lastMonth=%s", userName, modelHash, record.ResetMonth)
		return initialCredits, nil
	}

	return record.Credits, nil
}

// CheckCredits 检查用户对指定模型的可用积分是否充足（base + bonus > 0）
// 同时触发 GetOrInit，保证基础积分是本月的正确值
func CheckCredits(ctx *gin.Context, userName, modelHash string) error {
	baseCredits, err := GetOrInit(ctx, userName, modelHash)
	if err != nil {
		return err
	}

	bonusRecord, err := model.GetBonusCredits(ctx, userName, modelHash)
	if err != nil {
		return err
	}
	bonusCredits := 0
	if bonusRecord != nil {
		bonusCredits = bonusRecord.Credits
	}

	total := baseCredits + bonusCredits
	zlog.Infof(ctx, "[credits.CheckCredits] userName=%s, modelHash=%s, base=%d, bonus=%d, total=%d",
		userName, modelHash, baseCredits, bonusCredits, total)

	if total <= 0 {
		return components.ErrorCreditsInsufficient
	}
	return nil
}

// DeductCredits 异步扣减积分：优先扣赠送积分，不足部分扣基础积分，积分可扣为负数
// 换算：输入 5000 token = 1 积分，输出 1000 token = 1 积分
func DeductCredits(ctx context.Context, userName, modelHash string, inputTokens, outputTokens int) {
	amount := CalcDeduction(inputTokens, outputTokens)
	go func() {
		dbCtx := context.Background()
		if err := deductWithTransaction(dbCtx, userName, modelHash, amount); err != nil {
			zlog.Errorf(nil, "[credits.DeductCredits] 扣减失败: userName=%s, modelHash=%s, amount=%d, err=%v",
				userName, modelHash, amount, err)
		} else {
			zlog.Infof(nil, "[credits.DeductCredits] 扣减成功: userName=%s, modelHash=%s, amount=%d (input=%d, output=%d)",
				userName, modelHash, amount, inputTokens, outputTokens)
		}
	}()
}

// deductWithTransaction 事务内优先扣赠送积分，不足部分扣基础积分
func deductWithTransaction(ctx context.Context, userName, modelHash string, amount int) error {
	db := model.GetLLMDB()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		bonus, err := model.GetBonusCreditsForUpdate(tx, userName, modelHash)
		if err != nil {
			return err
		}

		bonusCredits := 0
		if bonus != nil {
			bonusCredits = bonus.Credits
		}

		if bonusCredits >= amount {
			// 赠送积分足够，全从赠送积分扣
			return tx.Model(&model.UserBonusCredits{}).
				Where("user_name = ? AND model_hash = ?", userName, modelHash).
				UpdateColumn("credits", gorm.Expr("credits - ?", amount)).Error
		}

		// 赠送积分不足：先清零赠送积分，剩余从基础积分扣
		remainder := amount - bonusCredits
		if bonusCredits > 0 {
			if err := tx.Model(&model.UserBonusCredits{}).
				Where("user_name = ? AND model_hash = ?", userName, modelHash).
				UpdateColumn("credits", 0).Error; err != nil {
				return err
			}
		}

		return tx.Model(&model.UserBaseCredits{}).
			Where("user_name = ? AND model_hash = ?", userName, modelHash).
			UpdateColumn("credits", gorm.Expr("credits - ?", remainder)).Error
	})
}

// AdjustCredits 调整指定用户的赠送积分（正数增加，负数减少），白名单用户专用
func AdjustCredits(ctx *gin.Context, userName, modelHash string, delta int) error {
	return model.UpsertBonusCredits(ctx, userName, modelHash, delta)
}
