import { useNavigate } from "@tanstack/solid-router";
import Headphones from "lucide-solid/icons/headphones";
import { createResource, For, Show } from "solid-js";
import { type Domain, getDomain } from "@/api/domain";
import DomainIcon from "@/components/domain/DomainIcon";
import { ManageLoading } from "@/components/manage/ManageShell";
import domainStore from "@/stores/domainStore";

function DiscoverMyDomains() {
	const navigate = useNavigate();
	const myUUIDs = () => domainStore.state.myDomainUUIDs;

	const [domains] = createResource(myUUIDs, async (uuids) => {
		const results = await Promise.allSettled(
			uuids.map(async (uuid) => {
				const cached = domainStore.state.domainCache[uuid];
				if (cached) return cached;
				return getDomain(uuid);
			}),
		);
		return results
			.filter(
				(r): r is PromiseFulfilledResult<Domain> => r.status === "fulfilled",
			)
			.map((r) => r.value);
	});

	const openDomain = (domain: Domain) => {
		domainStore.setCurrentDomain(domain.uuid);
		void navigate({
			to: "/domain/$domainUUID",
			params: { domainUUID: domain.uuid },
		});
	};

	return (
		<div class="p-4 md:p-5">
			<Show when={domainStore.state.loading && myUUIDs().length === 0}>
				<ManageLoading label="正在加载我的域" />
			</Show>

			<Show when={!domainStore.state.loading && myUUIDs().length === 0}>
				<div class="py-6 text-sm text-base-content/50">暂无已加入的域</div>
			</Show>

			<Show when={myUUIDs().length > 0 && !domains()}>
				<ManageLoading label="正在加载我的域" />
			</Show>

			<Show when={domains()?.length}>
				<div class="grid gap-2 md:grid-cols-2">
					<For each={domains()}>
						{(domain) => (
							<div class="flex min-w-0 items-center gap-3 rounded-xl border border-base-300/80 bg-base-200/40 px-3 py-3 transition-colors hover:border-primary/30 hover:bg-base-100">
								<DomainIcon
									name={domain.name}
									iconUrl={domain.icon_url}
									class="shrink-0"
								/>
								<div class="min-w-0 flex-1">
									<div class="truncate text-sm font-medium">{domain.name}</div>
									<div class="mt-0.5 truncate text-xs text-base-content/50">
										{domain.description || "暂无描述"}
									</div>
								</div>
								<button
									type="button"
									class="btn btn-outline btn-sm shrink-0"
									onClick={() => openDomain(domain)}
								>
									<Headphones size={14} />
									进入
								</button>
							</div>
						)}
					</For>
				</div>
			</Show>
		</div>
	);
}

export default DiscoverMyDomains;
