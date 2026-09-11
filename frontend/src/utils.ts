export function timestamp(value?: number) {
  if (!value) return "-";
  return new Date(value * 1000).toLocaleString();
}

// qqAvatarUrl 通过 QQ 号生成 QQ 头像地址；非合法 QQ 号返回空串（调用方回退到文字头像）。
export function qqAvatarUrl(qq?: string) {
  const value = (qq || "").trim();
  if (!/^\d{5,12}$/.test(value)) return "";
  return `https://q2.qlogo.cn/headimg_dl?dst_uin=${value}&spec=100`;
}
