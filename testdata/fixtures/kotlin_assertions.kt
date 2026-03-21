package com.example

import org.junit.Test
import io.mockk.verify
import io.mockk.every

class AssertionTest {
    @Test
    fun `strong assertions`() {
        assertEquals(expected, actual)
        assertThat(result).isEqualTo(expected)
        assertThat(list).contains(item)
    }

    @Test
    fun `medium assertions`() {
        assertTrue(flag)
        assertFalse(condition)
    }

    @Test
    fun `weak assertion`() {
        assertNotNull(result)
    }

    @Test
    fun `exception assertion`() {
        assertThrows<IllegalArgumentException> { sut.doThing() }
    }

    @Test
    fun `verify calls - interaction`() {
        verify { repo.save(user) }
        verify { analytics.track(event) }
    }

    @Test
    fun `zero assertion method`() {
        sut.doSomething()
        // no assertions
    }
}
