<script lang="ts">

    interface Props {
        status: number;
        message?: string;
    }

    const { status, message }: Props = $props();
</script>

<div class="center-container">
    <div class="panel" style="width: 550px; height: 550px; text-align: center">
        <div class="text-xlg">
            Error {status}
        </div>
        <p class="text-xsm">
            {message || ''} Return to the <a class="highlight" href="/">homepage</a>?
        </p>
        <img alt="Error" src="/static/images/error-image.jpg" style="margin: 20px; width: 500px; height: auto;" />
    </div>
</div>
