import AlertTriangle from "lucide-solid/icons/alert-triangle";
import RefreshCw from "lucide-solid/icons/refresh-cw";
import { createMemo } from "solid-js";

interface ErrorComponentProps {
	error: Error;
	reset?: () => void;
}

const ErrorComponent = ({ error, reset }: ErrorComponentProps) => {
	const title = createMemo(() => {
		if (!error) return "页面加载失败";
		return error.name || "页面加载失败";
	});

	const handleReset = () => {
		if (typeof reset === "function") {
			reset();
			return;
		}
		window.location.reload();
	};

	return (
		<div class="flex min-h-screen w-full items-center justify-center bg-base-200 p-4">
			<div class="w-full max-w-md rounded-lg border border-base-300 bg-base-100 p-6 text-center shadow-sm">
				<div class="mx-auto mb-4 flex size-12 items-center justify-center rounded-lg bg-error/10 text-error">
					<AlertTriangle size={24} />
				</div>
				<h1 class="text-lg font-semibold">{title()}</h1>
				<p class="mt-2 break-words text-sm leading-6 text-base-content/65">
					{error?.message || "发生了未知错误，请重试。"}
				</p>
				<button
					type="button"
					class="btn btn-primary mt-5 min-w-28"
					onClick={handleReset}
				>
					<RefreshCw size={16} />
					重试
				</button>
			</div>
		</div>
	);
};
export default ErrorComponent;
