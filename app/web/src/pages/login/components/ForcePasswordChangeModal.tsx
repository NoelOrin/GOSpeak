import {
	Dialog,
	DialogContent,
	DialogHeader,
	DialogTitle,
} from "@gospeak/components";
import PasswordChangeForm from "@/components/form/PasswordChangeForm";

export interface ForcePasswordChangeModalProps {
	open: boolean;
	onClose: () => void;
	onSubmit: (values: {
		username?: string;
		oldPassword?: string;
		newPassword: string;
		name?: string;
	}) => Promise<void>;
}

export default function ForcePasswordChangeModal({
	open,
	onClose,
	onSubmit,
}: ForcePasswordChangeModalProps) {
	if (!open) return null;

	return (
		<Dialog open onOpenChange={(v) => !v && onClose()}>
			<DialogContent>
				<DialogHeader>
					<DialogTitle>修改密码</DialogTitle>
				</DialogHeader>
				<p className="text-muted-foreground text-sm">
					检测到您使用的是默认密码，请修改密码后继续。
				</p>
				<PasswordChangeForm
					showOldPassword={false}
					showName={true}
					submitText="修改密码"
					onSubmit={onSubmit}
				/>
			</DialogContent>
		</Dialog>
	);
}
