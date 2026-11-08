import { Input } from "@gospeak/components";
import { Eye, EyeOff } from "lucide-react";
import { forwardRef, useState } from "react";

type PasswordInputProps = React.ComponentPropsWithoutRef<"input"> & {
	/**
	 * 渲染不带组件库默认样式的原生 input。
	 * 管理后台有一批表单用自定义 class 画输入框，直接套 Input 会叠出双层边框。
	 */
	native?: boolean;
};

/**
 * 带「显示/隐藏」切换的密码输入框。
 *
 * 全站密码框原先一律固定 type="password"，没有可见性切换 —— 手机端输长密码
 * 基本靠猜。这里统一一个组件，顺带把 autoComplete 的默认值收敛到 new-password
 * （登录页会显式传 current-password 覆盖）。
 */
const PasswordInput = forwardRef<HTMLInputElement, PasswordInputProps>(
	(
		{ className, autoComplete = "new-password", native = false, ...props },
		ref,
	) => {
		const [visible, setVisible] = useState(false);
		const inputType: "text" | "password" = visible ? "text" : "password";
		const shared = {
			...props,
			ref,
			type: inputType,
			autoComplete,
			className: `pr-9 ${className ?? ""}`,
		};
		return (
			<div className="relative w-full">
				{native ? <input {...shared} /> : <Input {...shared} />}
				<button
					type="button"
					className="text-muted-foreground hover:text-foreground absolute top-1/2 right-1 flex size-7 -translate-y-1/2 cursor-pointer items-center justify-center rounded-sm transition-colors"
					onClick={() => setVisible((v) => !v)}
					aria-label={visible ? "隐藏密码" : "显示密码"}
					title={visible ? "隐藏密码" : "显示密码"}
					tabIndex={-1}
				>
					{visible ? <EyeOff size={14} /> : <Eye size={14} />}
				</button>
			</div>
		);
	},
);
PasswordInput.displayName = "PasswordInput";

export default PasswordInput;
