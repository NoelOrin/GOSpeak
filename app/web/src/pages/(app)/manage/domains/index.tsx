import { createFileRoute, Link } from "@tanstack/solid-router";
import { createResource, For, Show } from "solid-js";
import { getDomain, myDomains } from "@/api/domain";
import {
	ManageHeader,
	ManagePage,
	ManageSection,
} from "@/components/manage/ManageShell";

export const Route = createFileRoute("/(app)/manage/domains/")({
	component: RouteComponent,
	staticData: { title: "域管理", icon: "icon-manage" },
});

function RouteComponent() {
	const [uuids] = createResource(myDomains);
	const [domains] = createResource(uuids, async (ids) => {
		const settled = await Promise.allSettled(ids.map((id) => getDomain(id)));
		return settled
			.filter(
				(
					r,
				): r is PromiseFulfilledResult<Awaited<ReturnType<typeof getDomain>>> =>
					r.status === "fulfilled",
			)
			.map((r) => r.value);
	});

	return (
		<ManagePage>
			<ManageHeader title="域管理" description="管理我加入的语音域" />
			<ManageSection title="我的域" padded={false}>
				<Show
					when={domains()?.length}
					fallback={<div class="p-5 text-sm text-base-content/50">暂无域</div>}
				>
					<div class="divide-y divide-base-200">
						<For each={domains()}>
							{(domain) => (
								<div class="flex items-center justify-between gap-3 px-4 py-3">
									<div class="min-w-0">
										<div class="font-medium truncate">{domain.name}</div>
										<div class="text-xs text-base-content/50 truncate">
											{domain.description || "暂无描述"}
										</div>
									</div>
									<Link
										to="/manage/domains/$domainUUID"
										params={{ domainUUID: domain.uuid }}
										class="btn btn-sm btn-outline"
									>
										管理
									</Link>
								</div>
							)}
						</For>
					</div>
				</Show>
			</ManageSection>
		</ManagePage>
	);
}
