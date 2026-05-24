import { createFileRoute } from "@tanstack/solid-router";
import Layout from "@/layouts/layout";

export const Route = createFileRoute("/(app)/")({
	component: RouteComponent,
});

function RouteComponent() {
	return (
		<Layout>
			<div class="flex justify-center items-center h-full">
				<div class="gap-4 grid grid-cols-4">
					<div className="shadow-sm card">
						<div className="card-body">123</div>
					</div>
					<div className="shadow-sm card">
						<div className="card-body">123</div>
					</div>
					<div className="shadow-sm card">
						<div className="card-body">123</div>
					</div>
				</div>
			</div>
		</Layout>
	);
}
