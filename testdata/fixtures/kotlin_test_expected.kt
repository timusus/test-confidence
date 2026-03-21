package com.example

import org.junit.Test

class TestExpectedTest {
    @Test(expected = IllegalArgumentException::class)
    fun `throws exception`() {
        sut.doThing()
    }

    @Test
    fun `normal test with assertion`() {
        assertThat(result).isEqualTo(42)
    }

    @Test(expected = NullPointerException::class)
    fun `another expected exception`() {
        sut.processNull()
    }
}
