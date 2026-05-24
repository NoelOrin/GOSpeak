const OptionSquare = ({
	children,
}: {
	children: JSX.Element;
}) => {
	return (
		<div class="tooltip-right tooltip">
			<div class="tooltip-content">
				<div class="font-black text-orange-400 text-2xl -rotate-10 animate-bounce">
					Wow!
				</div>
			</div>
			<div class="flex justify-center items-center border border-gray-300 rounded-lg w-12 h-12">
				{children}
			</div>
		</div>
	);
};

export default OptionSquare;