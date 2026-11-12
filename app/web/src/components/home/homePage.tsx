import QuickActions from "@/components/dashboard/quick-actions";

export default function HomePage() {
	return (
		<div className="bg-card flex h-full min-h-0 min-w-0 flex-col overflow-y-auto">
			<div className="min-w-0 px-2 py-1">
				<QuickActions compact />
			</div>
		</div>
	);
}
