import * as DialogPrimitive from "@radix-ui/react-dialog";
import { X } from "lucide-react";
import { forwardRef } from "react";
import { cn } from "../lib/utils";

export const Dialog = DialogPrimitive.Root;
export const DialogTrigger = DialogPrimitive.Trigger;
export const DialogClose = DialogPrimitive.Close;

export const DialogContent = forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Content>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Content> & {
		showClose?: boolean;
	}
>(({ className, children, showClose = true, ...props }, ref) => (
	<DialogPrimitive.Portal>
		<DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/50" />
		<DialogPrimitive.Content
			ref={ref}
			className={cn(
				"fixed top-1/2 left-1/2 z-50 grid w-full max-w-lg -translate-x-1/2 -translate-y-1/2 gap-4 rounded-2xl border border-border bg-card p-6 text-card-foreground shadow-xl",
				className,
			)}
			{...props}
		>
			{children}
			{showClose && (
				<DialogPrimitive.Close className="absolute top-4 right-4 flex size-7 items-center justify-center rounded-sm opacity-70 transition-opacity hover:bg-muted hover:opacity-100 focus:outline-none">
					<X className="size-4" />
					<span className="sr-only">关闭</span>
				</DialogPrimitive.Close>
			)}
		</DialogPrimitive.Content>
	</DialogPrimitive.Portal>
));
DialogContent.displayName = "DialogContent";

export const DialogHeader = ({
	className,
	...props
}: React.HTMLAttributes<HTMLDivElement>) => (
	<div
		className={cn("flex flex-col gap-1.5 text-center sm:text-left", className)}
		{...props}
	/>
);

export const DialogTitle = forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Title>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Title>
>(({ className, ...props }, ref) => (
	<DialogPrimitive.Title
		ref={ref}
		className={cn("text-lg leading-none font-semibold", className)}
		{...props}
	/>
));
DialogTitle.displayName = "DialogTitle";

export const DialogDescription = forwardRef<
	React.ComponentRef<typeof DialogPrimitive.Description>,
	React.ComponentPropsWithoutRef<typeof DialogPrimitive.Description>
>(({ className, ...props }, ref) => (
	<DialogPrimitive.Description
		ref={ref}
		className={cn("text-sm text-muted-foreground", className)}
		{...props}
	/>
));
DialogDescription.displayName = "DialogDescription";
