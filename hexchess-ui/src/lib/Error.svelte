<script lang="ts">
	interface Props {
		status: number;
		message?: string;
	}

	const { status, message }: Props = $props();
</script>

<div class="center-container">
	<div class="panel" style="width: 600px; height: 600px; text-align: center">
		<div class="text-xlg">
			Error {status}
		</div>
		<p class="text-xsm">
			{message || ''}
		</p>
		<p class="text-xsm">
			Return to the <a class="highlight" href="/static">homepage</a>?
		</p>
		<img alt="Error" src="/error-image.jpg" style="margin: 20px; width: 550px; height: auto;" />
	</div>
</div>
