package com.example

import org.junit.Test
import app.cash.turbine.test

class TurbineAssertionTest {
    @Test
    fun `turbine test with multiple assertions`() {
        repository.observeNextEpisodeToWatch(showId).test {
            assertThat(awaitItem()?.episode).isEqualTo(s1e1)
            assertThat(awaitItem()?.episode).isEqualTo(s1e2)
            assertThat(awaitItem()?.episode).isEqualTo(s1e3)
        }
    }

    @Test
    fun `turbine test with shouldBe assertions`() {
        flow.test {
            awaitItem() shouldBe expected1
            awaitItem() shouldBe expected2
        }
    }

    @Test
    fun `custom assertion helper`() {
        assertColorSchemesEqual(expected, actual)
        verifySnapshot(component)
        validateResponse(response)
    }

    @Test
    fun `mixed turbine and regular`() {
        val result = sut.compute()
        assertThat(result).isEqualTo(42)
        flow.test {
            assertThat(awaitItem()).isEqualTo(first)
            assertThat(awaitItem()).isEqualTo(second)
        }
    }
}
