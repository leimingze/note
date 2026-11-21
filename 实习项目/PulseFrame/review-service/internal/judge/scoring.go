package judge

// 输入：一次有效采样的回答。
// 输出：保留该次观察、标签、证据及主动转审请求的判定。
// 功能：将一次回答单独交给规则表，例如暴力证据只影响报告它的那一票。
func FromOutput(out Output) Decision {
	d := Decision{Observations: out.Observations, ViolenceDetected: out.ViolenceDetected,
		OtherLabels: out.OtherLabels, IsSwimwearProduct: out.IsSwimwearProduct,
		NeedHumanReview: out.NeedHumanReview, Evidence: out.Evidence}
	if out.NeedHumanReview {
		d.HumanReasons = []string{"模型要求复核"}
	}
	return d
}
