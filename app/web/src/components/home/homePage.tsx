import QuickActions from "@/components/dashboard/quick-actions";

const HomePage = () => {
	return (
		<div class="flex h-full min-h-0 min-w-0 flex-col overflow-y-auto bg-base-100">
			<div class="min-w-0 px-2 py-1">
				<QuickActions compact />
			</div>
		</div>
	);
};

export default HomePage;
